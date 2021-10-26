package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const keyTokens = "user_tokens"

var errNoTokens = errors.New("tokens not found")

type MW struct {
	SM *scs.SessionManager
}

func (mw *MW) GetTokens(c *gin.Context) (*oauth2.Token, error) {
	bs := mw.SM.GetBytes(c.Request.Context(), keyTokens)
	if bs == nil {
		return nil, errNoTokens
	}
	var tok oauth2.Token
	if err := json.Unmarshal(bs, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (mw *MW) StoreTokens(c *gin.Context, tok *oauth2.Token) error {
	bs, err := json.Marshal(tok)
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
		c.Next()
	case errNoTokens:
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	default:
		c.AbortWithError(500, err)
		fmt.Fprint(c.Writer, "an unexpected error occurred")
	}
}

func (mw *MW) Logout(c *gin.Context) {
	if err := mw.SM.Destroy(c.Request.Context()); err != nil {
		c.Error(fmt.Errorf("destroying session: %w", err))
	}
}
