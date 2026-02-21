package main

import (
	"context"
	"fmt"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	mailer "raven/auth/Mailer"
	"time"

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

	if user.IsDeleted == 1 {
		c.JSON(403, gin.H{"success": false, "reason": "Invalid credentials"})
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

	sessionData := map[string]interface{}{ //Data that will be saved in Redis.
		"refresh_token": refreshToken,
		"user_id":       user.UserID,
		"blacklisted":   false,
	}

	a.userRedis.InsertToken(context.Background(), sessionData, 24*time.Hour) //Saves the refresh token in Redis with an expiry of 7 days

	c.JSON(200, gin.H{
		"success":       true,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	})

}

func (a *App) handleRefresh(c *gin.Context) {
	var requestData struct {
		RefreshToken string `json:"refresh_token"`
	}

	err := c.ShouldBindJSON(&requestData)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid refresh token"})
		return
	}

	payload, err := auth.ValidateToken(requestData.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid refresh token"})
		return
	}

	fmt.Println(payload)
}

func (a *App) dbtest(c *gin.Context) { //Tests user authentication
	userID, exists := c.Get("user_id")

	if exists != true {
		c.JSON(400, gin.H{"error": "user_id not found in context"})
		return
	}

	fmt.Printf("User ID from context: %v\n", userID)

	user := a.Ur.GetAccountById(userID.(int))

	if user == nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}
	c.JSON(200, gin.H{"user": &user})
}

// Shows the user the current version of the API
func handleAbout(c *gin.Context) {
	c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", config.Version, config.Build)})
}
