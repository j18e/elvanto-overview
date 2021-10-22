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

func GetTokens(sm *scs.SessionManager, c *gin.Context) (*models.Tokens, error) {
	bs := sm.GetBytes(c.Request.Context(), keyTokens)
	if bs == nil {
		return nil, errNoTokens
	}
	var tok models.Tokens
	if err := json.Unmarshal(bs, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func StoreTokens(sm *scs.SessionManager, c *gin.Context, tok models.Tokens) error {
	bs, err := json.Marshal(&tok)
	if err != nil {
		return err
	}
	sm.Put(c.Request.Context(), keyTokens, bs)
	return nil
}

func RequireTokens(sm *scs.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := GetTokens(sm, c)
		switch err {
		case nil:
			c.Next()
		case errNoTokens:
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
		default:
			c.AbortWithError(500, err)
			fmt.Fprint(c.Writer, "an unexpected error occurred")
		}
	}
}

func RefreshTokens(cli *http.Client, sm *scs.SessionManager) gin.HandlerFunc {
	const url = "https://api.elvanto.com/oauth/token"

	return func(c *gin.Context) {
		c.Next()
		log.Debug("refreshing tokens")
		oldTok, _ := GetTokens(sm, c)

		vals := make(urlpkg.Values)
		vals.Set("grant_type", "refresh_token")
		vals.Set("refresh_token", oldTok.Refresh)

		res, err := cli.PostForm(url, vals)
		if err != nil {
			log.Errorf("refreshing tokens: posting request: %s", err)
			return
		}
		defer res.Body.Close()

		if res.StatusCode > 399 {
			log.Errorf("refreshing tokens: got status %s", res.Status)
			return
		}

		var newTok models.Tokens
		if err := json.NewDecoder(res.Body).Decode(&newTok); err != nil {
			log.Errorf("refreshing tokens: decoding response: %s", err)
			return
		}
		if err := StoreTokens(sm, c, newTok); err != nil {
			log.Errorf("refreshing tokens: storing new tokens: %s", err)
			return
		}
		c.Set(keyTokens, newTok)
		log.Debug("finished refreshing tokens")
	}
}
