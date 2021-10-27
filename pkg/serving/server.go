package serving

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/j18e/elvanto-overview/pkg/middleware"
	"github.com/j18e/elvanto-overview/pkg/models"
)

const (
	elvantoAPI = "https://api.elvanto.com/v1"

	tplOverview  = "overview.html"
	tplUser      = "user.html"
	tplLoggedOut = "logged_out.html"
)

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

		var svcTypes []models.ServiceType
		if err := json.Unmarshal(bs, &svcTypes); err != nil {
			c.AbortWithError(500, err)
			return
		}
		data := overviewData{
			Services:      svcTypes,
			ElvantoDomain: elvantoDomain,
		}
		c.HTML(200, tplOverview, data)
	}
}

func (s *Server) HandleUser(c *gin.Context) {
	user := s.MW.GetUser(c)
	if user == nil {
		tok := s.MW.GetTokens(c)
		var err error
		user, err = s.currentUser(tok)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		s.MW.StoreUser(c, *user)
	}
	c.HTML(200, tplUser, user)
}

func (s *Server) HandleOverview(c *gin.Context) {
	tok := s.MW.GetTokens(c)
	services, err := s.loadServices(tok)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	data := overviewData{
		Services:      services,
		ElvantoDomain: s.ElvantoDomain,
	}
	c.HTML(200, tplOverview, data)
}

func (s *Server) HandleLoggedOut(c *gin.Context) {
	c.HTML(200, tplLoggedOut, map[string]bool{"LoggedOut": true})
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
	s.MW.StoreTokens(c, tok)
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) loadServices(tok *oauth2.Token) ([]models.ServiceType, error) {
	const url = elvantoAPI + "/services/getAll.json?page=1&page_size=100&status=published&fields[0]=volunteers"
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

	var stl models.ServiceTypeList
	if err := json.NewDecoder(res.Body).Decode(&stl); err != nil {
		return nil, err
	}
	return stl, nil
}

func (s *Server) currentUser(tok *oauth2.Token) (*models.User, error) {
	const url = elvantoAPI + "/people/currentUser.json"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	cli := s.Oauth2.Client(context.Background(), tok)
	res, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		return nil, fmt.Errorf("get current user: got status %d", res.StatusCode)
	}
	var u models.User
	if err := json.NewDecoder(res.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	return &u, nil
}
