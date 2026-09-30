package middleware

import (
	"net/http"
	"strings"

	"com.geoffjay.track/internal/auth"

	"github.com/gin-gonic/gin"
)

// APIAuth authenticates /api/v1 requests via either mechanism:
//
//   - Authorization: Bearer <token> — an API token created on the settings
//     page (verified against the api_tokens table, expiry enforced)
//   - HTTP basic auth — the same credentials the web UI uses
//
// Unauthenticated requests get 401 with a WWW-Authenticate header
// advertising both schemes.
func APIAuth(store *auth.Store, tokens *auth.TokenStore, realm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Bearer token takes precedence when the header carries one.
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			user, err := tokens.Verify(c.Request.Context(), raw)
			if err == nil {
				c.Set(userKey, &user)
				c.Next()
				return
			}
			c.Header("WWW-Authenticate", `Bearer realm="`+realm+`", charset="UTF-8"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Fall through to basic auth (same semantics as the UI).
		if username, password, ok := c.Request.BasicAuth(); ok {
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
