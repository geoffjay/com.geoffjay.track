// Package middleware provides gin middleware for HTTP basic authentication
// backed by the auth store.
package middleware

import (
	"net/http"

	"com.geoffjay.track/internal/auth"

	"github.com/gin-gonic/gin"
)

// userKey is the gin context key holding the authenticated *auth.User.
const userKey = "track.user"

// BasicAuth returns gin middleware that enforces HTTP basic auth against the
// user store. On success the user is stored in the context (retrieve with
// UserFrom). Unauthenticated requests get a 401 with the WWW-Authenticate
// header so the browser shows its native login prompt.
func BasicAuth(store *auth.Store, realm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if ok {
			user, err := store.Authenticate(c.Request.Context(), username, password)
			if err == nil {
				c.Set(userKey, &user)
				c.Next()
				return
			}
		}
		c.Header("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}

// UserFrom returns the authenticated user from the request context, or nil.
func UserFrom(c *gin.Context) *auth.User {
	if v, ok := c.Get(userKey); ok {
		if u, ok := v.(*auth.User); ok {
			return u
		}
	}
	return nil
}