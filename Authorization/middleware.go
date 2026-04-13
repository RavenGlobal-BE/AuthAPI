package authorization

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	dbEngine "raven/auth/DatabaseEngine"
	logger "raven/auth/Logging"
	"strconv"

	"github.com/gin-gonic/gin"
)

/* As of v26.2 RC2, cookies are now a valid form of authorization. */
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		cookie, err := c.Cookie("access_token")

		logger.Log(fmt.Sprintf("Auth Header: %s, Cookie: %s", authHeader, cookie), logger.Debug)
		tokenString := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else if err == nil && cookie != "" {
			tokenString = cookie
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid credentials"})
			return
		}
		payload, err := ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if payload.TokenType != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		userID, err := strconv.Atoi(payload.RegisteredClaims.Subject) // Convert userID from string to int
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("user_id", userID)
		c.Set("email", payload.Email)
		c.Set("session_id", payload.SessionID)
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
