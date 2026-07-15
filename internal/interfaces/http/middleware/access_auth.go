package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AccessAuth(token string) gin.HandlerFunc {
	configuredToken := strings.TrimSpace(token)

	return func(c *gin.Context) {
		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		presentedToken := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		valid := configuredToken != "" &&
			strings.HasPrefix(auth, prefix) &&
			presentedToken != "" &&
			subtle.ConstantTimeCompare([]byte(presentedToken), []byte(configuredToken)) == 1
		if !valid {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"code":    "unauthorized",
				"message": "valid bearer access token is required",
			}})
			return
		}
		c.Next()
	}
}
