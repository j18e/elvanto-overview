package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	bs, err := ioutil.ReadFile("apikey")
	if err != nil {
		return err
	}
	apikey := strings.TrimSpace(string(bs))

	services, err := loadServices(apikey)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.NewTicker(time.Hour).C:
				res, err := loadServices(apikey)
				if err != nil {
					log.Errorf("loading services: %v", err)
					continue
				}
				services = res
			}
		}
	}()

	r := gin.Default()
	r.LoadHTMLGlob("template.html")
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "template.html", services)
	})
	r.POST("/reload", func(c *gin.Context) {
		res, err := loadServices(apikey)
		if err != nil {
			c.AbortWithError(500, fmt.Errorf("loading services: %v", err))
			return
		}
		services = res
		c.Redirect(http.StatusFound, "/")
	})
	listenAddr := ":3000"
	log.Infof("listening on %s", listenAddr)
	return r.Run(listenAddr)
}

func loadServices(apikey string) ([]ServiceType, error) {
	url := "https://api.elvanto.com/v1/services/getAll.json?page=1&page_size=100&status=published&fields[0]=volunteers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(apikey, "x")

	req.Form = make(urlpkg.Values)

	cli := &http.Client{Timeout: time.Second * 10}
	res, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		return nil, fmt.Errorf("making request: got status %d", res.StatusCode)
	}
	bs, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var data ServicesResponse
	if err := json.Unmarshal(bs, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}
	services := make(map[string][]Service)
	for _, svc := range data.Services.Service {
		service := Service{
			Name:       svc.Name,
			Location:   svc.Location.Name,
			Date:       svc.Date,
			Volunteers: svc.Volunteers,
		}
		services[svc.Type.Name] = append(services[svc.Type.Name], service)
	}
	var serviceTypes []ServiceType
	for t, sx := range services {
		serviceTypes = append(serviceTypes, ServiceType{
			Type:     t,
			Services: sx,
		})
	}

	sort.Slice(serviceTypes, func(i, j int) bool {
		return serviceTypes[i].Type < serviceTypes[j].Type
	})

	return serviceTypes, nil
}

type ServiceType struct {
	Type     string
	Services []Service
}

type Service struct {
	Name       string
	Location   string
	Date       string
	Volunteers []Volunteer
}

func (s Service) String() string {
	res := fmt.Sprintf("%s: %s at %s:", s.Date, s.Name, s.Location)
	for _, v := range s.Volunteers {
		res += "\n\t" + v.String()
	}
	return res
}

type Volunteer struct {
	Name       string
	Department string
	Position   string
}

func (v Volunteer) String() string {
	return fmt.Sprintf("%s/%s: %s", v.Department, v.Position, v.Name)
}
