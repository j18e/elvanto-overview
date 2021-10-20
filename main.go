package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/j18e/elvanto-overview/pkg/models"
	"github.com/j18e/elvanto-overview/pkg/repositories"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

const cookieTokenID = "token_id"

type Config struct {
	ClientID     string `required:"true" envconfig:"CLIENT_ID"`
	ClientSecret string `required:"true" envconfig:"CLIENT_SECRET"`
	RedirectURI  string `required:"true" envconfig:"REDIRECT_URI"`
	Domain       string `required:"true" envconfig:"DOMAIN"`
	DataFile     string `required:"true" envconfig:"DATA_FILE"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var conf Config
	if err := envconfig.Process("", &conf); err != nil {
		return err
	}

	repo, err := repositories.NewRepository(conf.DataFile)
	if err != nil {
		return err
	}

	srv := &Server{
		ClientID:     conf.ClientID,
		ClientSecret: conf.ClientSecret,
		RedirectURI:  conf.RedirectURI,
		Domain:       conf.Domain,
		httpCli:      &http.Client{Timeout: time.Second * 10},
		Repository:   repo,
	}

	r := gin.Default()
	r.LoadHTMLGlob("template.html")
	r.GET("/", srv.handleOverview)
	r.GET("/login", srv.login)
	r.GET("/login/complete", srv.completeLogin)
	listenAddr := ":3000"
	log.Infof("listening on %s", listenAddr)
	return r.Run(listenAddr)
}

type Server struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Domain       string
	httpCli      *http.Client
	Repository   *repositories.Repository
}

func (s *Server) handleOverview(c *gin.Context) {
	tokenID, err := c.Cookie(cookieTokenID)
	if err == http.ErrNoCookie {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	tok, err := s.Repository.Get(tokenID)
	if err == repositories.ErrNotFound {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	services, err := s.loadServices(tok.Access)
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

	var tok repositories.Tokens
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		c.AbortWithError(500, fmt.Errorf("decoding json: %w", err))
		return
	}
	if err := s.Repository.Store(data.Code, tok); err != nil {
		c.AbortWithError(500, errors.New("empty refresh token"))
		return
	}
	c.SetCookie(cookieTokenID, data.Code, 0, "", s.Domain, true, true)
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) loadServices(token string) ([]models.ServiceType, error) {
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

	var data models.ServicesResponse
	if err := json.Unmarshal(bs, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}
	services := make(map[string][]models.Service)
	for _, svc := range data.Services.Service {
		service := models.Service{
			Name:       svc.Name,
			Location:   svc.Location.Name,
			Date:       svc.Date,
			Volunteers: svc.Volunteers,
		}
		services[svc.Type.Name] = append(services[svc.Type.Name], service)
	}
	var serviceTypes []models.ServiceType
	for t, sx := range services {
		serviceTypes = append(serviceTypes, models.ServiceType{
			Type:     t,
			Services: sx,
		})
	}

	sort.Slice(serviceTypes, func(i, j int) bool {
		return serviceTypes[i].Type < serviceTypes[j].Type
	})

	return serviceTypes, nil
}
