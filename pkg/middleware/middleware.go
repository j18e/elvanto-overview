package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	urlpkg "net/url"

	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/j18e/elvanto-overview/pkg/models"
)

const keyTokens = "user_tokens"

var errNoTokens = errors.New("tokens not found")

type MW struct {
	HTTPCli *http.Client
	SM      *scs.SessionManager
}

func (mw *MW) GetTokens(c *gin.Context) (*models.TokenPair, error) {
	bs := mw.SM.GetBytes(c.Request.Context(), keyTokens)
	if bs == nil {
		return nil, errNoTokens
	}
	var tok models.TokenPair
	if err := json.Unmarshal(bs, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (mw *MW) StoreTokens(c *gin.Context, tok models.TokenPair) error {
	bs, err := json.Marshal(&tok)
	if err != nil {
		return err
	}
	mw.SM.Put(c.Request.Context(), keyTokens, bs)
	return nil
}

func (mw *MW) RequireTokens(c *gin.Context) {
	_, err := mw.GetTokens(c)
	switch err {
	case nil:
		go mw.refreshTokens(c.Copy())
		c.Next()
	case errNoTokens:
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	default:
		c.AbortWithError(500, err)
		fmt.Fprint(c.Writer, "an unexpected error occurred")
	}
}

func (mw *MW) refreshTokens(c *gin.Context) {
	const url = "https://api.elvanto.com/oauth/token"

	log.Debug("refreshing tokens")
	oldTok, _ := mw.GetTokens(c)

	vals := make(urlpkg.Values)
	vals.Set("grant_type", "refresh_token")
	vals.Set("refresh_token", oldTok.Refresh)

	res, err := mw.HTTPCli.PostForm(url, vals)
	if err != nil {
		log.Errorf("refreshing tokens: posting request: %s", err)
		return
	}
	defer res.Body.Close()

	if res.StatusCode > 399 {
		log.Errorf("refreshing tokens: got status %s", res.Status)
		return
	}

	var newTok models.TokenPair
	if err := json.NewDecoder(res.Body).Decode(&newTok); err != nil {
		log.Errorf("refreshing tokens: decoding response: %s", err)
		return
	}
	if err := mw.StoreTokens(c, newTok); err != nil {
		log.Errorf("refreshing tokens: storing new tokens: %s", err)
		return
	}
	log.Debug("finished refreshing tokens")
}
