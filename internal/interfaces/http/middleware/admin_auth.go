package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AdminAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(token) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "admin_auth_required",
				"message": "admin API token is not configured",
			}})
			return
		}

		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "admin_auth_required",
				"message": "admin authorization bearer token is required",
			}})
			return
		}

		got := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{
				"code":    "admin_auth_forbidden",
				"message": "admin authorization bearer token is invalid",
			}})
			return
		}

		c.Next()
	}
}
