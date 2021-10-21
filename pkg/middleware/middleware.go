package middleware

import (
	"encoding/json"
	"net/http"
	urlpkg "net/url"

	"github.com/gin-gonic/gin"
	"github.com/j18e/elvanto-overview/pkg/repositories"
	log "github.com/sirupsen/logrus"
)

const (
	cookieTokenID = "token_id"
	keyTokens     = "user_tokens"
	keyCode       = "user_tokens_code"
)

func RequireTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := GetTokens(c)
		if tok == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
	}
}

func SetTokens(repo *repositories.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		code, err := c.Cookie(cookieTokenID)
		if err != nil {
			return
		}
		if code == "" {
			return
		}
		tokens, err := repo.Get(code)
		if err != nil {
			return
		}
		c.Set(keyTokens, *tokens)
		c.Set(keyCode, code)
	}
}

func GetTokens(c *gin.Context) *repositories.Tokens {
	t, ok := c.Get(keyTokens)
	if !ok {
		return nil
	}
	tok, ok := t.(repositories.Tokens)
	if !ok {
		return nil
	}
	return &tok
}

func getCode(c *gin.Context) string {
	code, ok := c.Get(keyCode)
	if !ok {
		return ""
	}
	str, ok := code.(string)
	if !ok {
		return ""
	}
	return str
}

func RefreshTokens(cli *http.Client, repo *repositories.Repository) gin.HandlerFunc {
	const url = "https://api.elvanto.com/oauth/token"

	return func(c *gin.Context) {
		c.Next()
		log.Info("refreshing tokens")
		oldTok := GetTokens(c)
		code := getCode(c)
		if code == "" {
			return
		}

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

		var newTok repositories.Tokens
		if err := json.NewDecoder(res.Body).Decode(&newTok); err != nil {
			log.Errorf("refreshing tokens: decoding response: %s", err)
			return
		}
		if err := repo.Store(code, newTok); err != nil {
			log.Errorf("refreshing tokens: storing new tokens: %s", err)
			return
		}
		c.Set(keyTokens, newTok)
		log.Info("finished refreshing tokens")
	}
}
