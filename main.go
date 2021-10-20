package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"os"
	"sort"
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
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	redirectURI := os.Getenv("REDIRECT_URI")
	dataFile := os.Getenv("DATA_FILE")

	if clientID == "" {
		return errors.New("required env var CLIENT_ID")
	}
	if clientSecret == "" {
		return errors.New("required env var CLIENT_SECRET")
	}
	if redirectURI == "" {
		return errors.New("required env var REDIRECT_URI")
	}
	if dataFile == "" {
		return errors.New("required env var DATA_FILE")
	}

	srv := &Server{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		httpCli:      &http.Client{Timeout: time.Second * 10},
	}

	r := gin.Default()
	r.LoadHTMLGlob("template.html")
	r.GET("/login", srv.login)
	r.GET("/login/complete", srv.completeLogin)
	r.GET("/", srv.overview)
	listenAddr := ":3000"
	log.Infof("listening on %s", listenAddr)
	return r.Run(listenAddr)
}

type Server struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	httpCli      *http.Client
}

func (s *Server) overview(c *gin.Context) {
	tok := ""
	services, err := s.loadServices(tok)
	if err != nil {
		c.AbortWithError(500, err)
	}
	c.HTML(200, "template.html", services)
}

func (s *Server) login(c *gin.Context) {
	const uri = "https://api.elvanto.com/oauth?type=web_server&client_id=%s&redirect_uri=%s&scope=%s"
	c.Redirect(http.StatusFound, fmt.Sprintf(uri, s.ClientID, s.RedirectURI, "ManageServices"))
}

func (s *Server) completeLogin(c *gin.Context) {
	const uri = "https://api.elvanto.com/oauth/token"

	var data struct {
		Code string `form:"code"`
	}
	if err := c.ShouldBindQuery(&data); err != nil {
		c.String(400, "code not found")
		return
	}

	vals := make(urlpkg.Values)
	vals.Set("grant_type", "authorization_code")
	vals.Set("client_id", s.ClientID)
	vals.Set("client_secret", s.ClientSecret)
	vals.Set("code", data.Code)
	vals.Set("redirect_uri", s.RedirectURI)

	res, err := s.httpCli.PostForm(uri, vals)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("making request: %w", err))
		return
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		c.AbortWithError(500, fmt.Errorf("requesting token: status %s", res.Status))
		return
	}

	bs, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("reading body: %w", err))
		return
	}
	c.String(200, string(bs))
}

func (s *Server) loadServices(token string) ([]ServiceType, error) {
	url := "https://api.elvanto.com/v1/services/getAll.json?page=1&page_size=100&status=published&fields[0]=volunteers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth(token, "x")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := s.httpCli.Do(req)
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
