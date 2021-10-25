package serving

import (
	"encoding/json"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/j18e/elvanto-overview/pkg/middleware"
	"github.com/j18e/elvanto-overview/pkg/models"
)

const keyTokens = "user_tokens"

type Server struct {
	ClientID      string
	ClientSecret  string
	RedirectURI   string
	Domain        string
	ElvantoDomain string
	HTTPCli       *http.Client
	MW            middleware.MW
}

type overviewData struct {
	Services      []models.ServiceType
	ElvantoDomain string
}

func DryRunHandler(dataFile, elvantoDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		f, err := os.Open(dataFile)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer f.Close()

		svcTypes, err := models.RenderServices(f)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		data := overviewData{
			Services:      svcTypes,
			ElvantoDomain: elvantoDomain,
		}
		c.HTML(200, "template.html", data)
	}
}

func (s *Server) HandleOverview(c *gin.Context) {
	log.Debug("getting overview")
	tok, _ := s.MW.GetTokens(c)
	services, err := s.loadServices(tok.Access)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	data := overviewData{
		Services:      services,
		ElvantoDomain: s.ElvantoDomain,
	}
	c.HTML(200, "template.html", data)
	log.Debug("finished getting overview")
}

func (s *Server) HandleLogin(c *gin.Context) {
	const uri = "https://api.elvanto.com/oauth?type=web_server&client_id=%s&redirect_uri=%s&scope=%s"
	c.Redirect(http.StatusFound, fmt.Sprintf(uri, s.ClientID, s.RedirectURI, "ManageServices"))
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

	vals := make(urlpkg.Values)
	vals.Set("grant_type", "authorization_code")
	vals.Set("client_id", s.ClientID)
	vals.Set("client_secret", s.ClientSecret)
	vals.Set("code", data.Code)
	vals.Set("redirect_uri", s.RedirectURI)

	res, err := s.HTTPCli.PostForm(uri, vals)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("making request: %w", err))
		return
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		c.AbortWithError(500, fmt.Errorf("requesting token: status %s", res.Status))
		return
	}

	var tok models.TokenPair
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		c.AbortWithError(500, fmt.Errorf("decoding json: %w", err))
		return
	}
	if err := s.MW.StoreTokens(c, tok); err != nil {
		c.AbortWithError(500, fmt.Errorf("storing token: %w", err))
		return
	}
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

	res, err := s.HTTPCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		return nil, fmt.Errorf("making request: got status %d", res.StatusCode)
	}

	serviceTypes, err := models.RenderServices(res.Body)
	if err != nil {
		return nil, err
	}
	return serviceTypes, nil
}
