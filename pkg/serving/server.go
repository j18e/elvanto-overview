package serving

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/j18e/elvanto-overview/pkg/models"
)

func init() {
	gob.Register(&oauth2.Token{})
}

const (
	elvantoAPI = "https://api.elvanto.com/v1"

	tplOverview  = "overview.html"
	tplLoggedOut = "logged_out.html"

	cookieName = "elvanto_overview"
	keyToken   = "tokens"
)

var errNoTokens = errors.New("tokens not found")

type Server struct {
	Oauth2        oauth2.Config
	Domain        string
	ElvantoDomain string
	Store         *sessions.CookieStore
}

type overviewData struct {
	Services      []models.ServiceType
	ElvantoDomain string
}

func (s *Server) HandleOverview(c *gin.Context) {
	tok, err := s.getToken(c)
	if errors.Is(err, errNoTokens) {
		c.HTML(200, tplLoggedOut, map[string]bool{"LoggedOut": true})
		return
	}
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	tok, err = s.refreshTokenIfNeeded(c, tok)
	if err != nil {
		c.Error(fmt.Errorf("refreshing token: %w", err))
	}
	services, err := s.loadServices(c, tok)
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

func (s *Server) HandleLogout(c *gin.Context) {
	sess, err := s.Store.Get(c.Request, cookieName)
	if err != nil {
		c.AbortWithError(500, fmt.Errorf("getting session: %w", err))
		return
	}
	sess.Values[keyToken] = nil
	if err := sess.Save(c.Request, c.Writer); err != nil {
		c.AbortWithError(500, fmt.Errorf("saving session: %w", err))
	}
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
	if err := s.saveToken(c, tok); err != nil {
		c.AbortWithError(500, fmt.Errorf("saving tokens: %w", err))
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) loadServices(c *gin.Context, tok *oauth2.Token) ([]models.ServiceType, error) {
	const url = elvantoAPI + "/services/getAll.json?page=1&page_size=100&status=published&fields[0]=volunteers"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	cli := s.Oauth2.Client(c.Request.Context(), tok)
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

func (s *Server) saveToken(c *gin.Context, tok *oauth2.Token) error {
	sess, err := s.Store.Get(c.Request, cookieName)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	sess.Values[keyToken] = tok
	if err := sess.Save(c.Request, c.Writer); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	return nil
}

func (s *Server) getToken(c *gin.Context) (*oauth2.Token, error) {
	sess, err := s.Store.Get(c.Request, cookieName)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	iface := sess.Values[keyToken]
	tok, ok := iface.(*oauth2.Token)
	if !ok {
		return nil, errNoTokens
	}
	return tok, nil
}

func (s *Server) refreshTokenIfNeeded(c *gin.Context, tok *oauth2.Token) (*oauth2.Token, error) {
	sessExpiry := time.Second * time.Duration(s.Store.Options.MaxAge)
	if time.Until(tok.Expiry) > sessExpiry {
		return tok, nil
	}
	newTok, err := s.Oauth2.TokenSource(c.Request.Context(), tok).Token()
	if err != nil {
		return tok, err
	}
	if err := s.saveToken(c, tok); err != nil {
		return tok, err
	}
	return newTok, nil
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
