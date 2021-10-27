package serving

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/j18e/elvanto-overview/pkg/middleware"
	"github.com/j18e/elvanto-overview/pkg/models"
)

const keyTokens = "user_tokens"

type Server struct {
	Oauth2        oauth2.Config
	Domain        string
	ElvantoDomain string
	MW            middleware.MW
}

type overviewData struct {
	Services      []models.ServiceType
	ElvantoDomain string
}

func DryRunHandler(dataFile, elvantoDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		bs, err := ioutil.ReadFile(dataFile)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		svcTypes, err := models.RenderServices(bs)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		data := overviewData{
			Services:      svcTypes,
			ElvantoDomain: elvantoDomain,
		}
		c.HTML(200, "overview.html", data)
	}
}

func (s *Server) HandleNotSignedIn(c *gin.Context) {
	c.Status(200)
	fmt.Fprint(c.Writer, `<html><body><a href="/login">Sign in</a></body></html>`)
}

func (s *Server) HandleOverview(c *gin.Context) {
	tok, _ := s.MW.GetTokens(c)
	services, err := s.loadServices(tok)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	data := overviewData{
		Services:      services,
		ElvantoDomain: s.ElvantoDomain,
	}
	c.HTML(200, "overview.html", data)
}

func (s *Server) HandleLogout(c *gin.Context) {
}

func (s *Server) HandleLogin(c *gin.Context) {
	url := fmt.Sprintf("%s?type=web_server&client_id=%s&redirect_uri=%s&scope=%s",
		s.Oauth2.Endpoint.AuthURL, s.Oauth2.ClientID, s.Oauth2.RedirectURL, strings.Join(s.Oauth2.Scopes, ","))
	c.Redirect(http.StatusFound, url)
}

func (s *Server) HandleCompleteLogin(c *gin.Context) {
	const uri = "https://api.elvanto.com/oauth/token"

	var data struct {
		Code string `form:"code"`
	}
	if err := c.ShouldBindQuery(&data); err != nil {
		c.String(400, "code not found")
		return
	}

	tok, err := s.Oauth2.Exchange(context.Background(), data.Code)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("getting tokens: %w", err))
		return
	}
	if err := s.MW.StoreTokens(c, tok); err != nil {
		c.AbortWithError(500, fmt.Errorf("storing token: %w", err))
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) loadServices(tok *oauth2.Token) ([]models.ServiceType, error) {
	url := "https://api.elvanto.com/v1/services/getAll.json?page=1&page_size=100&status=published&fields[0]=volunteers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	cli := s.Oauth2.Client(context.Background(), tok)
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
		return nil, err
	}
	serviceTypes, err := models.RenderServices(bs)
	if err != nil {
		return nil, err
	}
	return serviceTypes, nil
}
