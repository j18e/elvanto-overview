package middleware

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/j18e/elvanto-overview/pkg/models"
	"golang.org/x/oauth2"
)

func init() {
	gob.Register(oauth2.Token{})
	gob.Register(models.User{})
}

const (
	keyTokens = "user_tokens"
	keyUser   = "user_profile"
)

var (
	errNoTokens = errors.New("tokens not found")
	errNoUser   = errors.New("user profile not found")
)

type MW struct {
	SM *scs.SessionManager
}

func (mw *MW) GetTokens(c *gin.Context) *oauth2.Token {
	tok, ok := mw.SM.Get(c.Request.Context(), keyTokens).(oauth2.Token)
	if !ok {
		return nil
	}
	return &tok
}

func (mw *MW) StoreTokens(c *gin.Context, tok *oauth2.Token) {
	mw.SM.Put(c.Request.Context(), keyTokens, *tok)
}

func (mw *MW) GetUser(c *gin.Context) *models.User {
	user, ok := mw.SM.Get(c.Request.Context(), keyUser).(models.User)
	if !ok {
		return nil
	}
	return &user
}

func (mw *MW) StoreUser(c *gin.Context, user models.User) {
	mw.SM.Put(c.Request.Context(), keyUser, user)
}

func (mw *MW) RequireTokens(c *gin.Context) {
	tok := mw.GetTokens(c)
	if tok == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	}
}

func (mw *MW) Logout(c *gin.Context) {
	if err := mw.SM.Destroy(c.Request.Context()); err != nil {
		c.Error(fmt.Errorf("destroying session: %w", err))
	}
}
