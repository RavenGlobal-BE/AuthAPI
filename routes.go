package main

import (
	"fmt"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	mailer "raven/auth/Mailer"

	//"time"

	"github.com/gin-gonic/gin"
)

// Returns both a Bearer token & a refresh token
func (a *App) handleLogin(c *gin.Context) {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := c.ShouldBindJSON(&loginData)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid request body"})
		return
	}

	mailValid := mailer.EmailIsValid(loginData.Email)
	if mailValid == false {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid email"})
		return
	}

	user := a.Ur.GetAccountByEmail(loginData.Email)
	if user == nil {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		return
	}

	result := auth.CheckPasswordHash(loginData.Password, user.Password)
	if result == false {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		return
	}

	// Generate JWT tokens
	accessToken, err := auth.GenerateAccessToken(user.UserID, user.Email, user.FirstName)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate access token"})
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user.UserID, user.Email)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate refresh token"})
		return
	}

	c.JSON(200, gin.H{
		"success":       true,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	})

}

// Shows the user the current version of the API
func handleAbout(c *gin.Context) {
	c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", config.Version, config.Build)})
}

func (a *App) dbtest(c *gin.Context) {
	user := a.Ur.GetAccountByEmail("imad@raven.co.com")

	if user == nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, gin.H{"user": &user})
}
