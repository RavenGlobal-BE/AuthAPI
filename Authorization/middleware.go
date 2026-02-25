package authorization

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	dbEngine "raven/auth/DatabaseEngine"
	"strconv"

	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		payload, err := ValidateToken(tokenString)

		if payload.TokenType != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		userID, err := strconv.Atoi(payload.RegisteredClaims.Subject)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("user_id", userID)
		c.Set("email", payload.Email)
		c.Next()
	}
}

func RateLimiting(rr *dbEngine.RateRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if clientIP == "::1" || clientIP == "127.0.0.1" {
			c.Header("X-RateLimit-Limit", "Unlimited")
			c.Header("X-RateLimit-Remaining", "Unlimited")

			c.Next()
			return
		}

		limit, err := rr.CheckLimit(c.Request.Context(), clientIP)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			return
		}

		if limit >= 15 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			return
		}

		newLimit, err := rr.Increment(c.Request.Context(), clientIP, 15*(60*time.Second))

		c.Header("X-RateLimit-Limit", "15")
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", 15-newLimit))

		c.Next()
	}
}
