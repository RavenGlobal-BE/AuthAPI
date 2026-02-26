package main

import (
	"context"
	"fmt"
	"net/url"
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
		Nonce    string `json:"nonce"`
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

	user := a.ur.GetAccountByEmail(loginData.Email)
	if user == nil || user.IsDeleted == 1 {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		return
	}

	result := auth.CheckPasswordHash(loginData.Password, user.Password)
	if result == false {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		return
	}

	// Generate JWT tokens

	accessToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "access", time.Now().Add(15*time.Minute), loginData.Nonce)
	refreshToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "refresh", time.Now().Add(14*24*time.Hour), loginData.Nonce)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate refresh token"})
		return
	}

	sessionData := map[string]interface{}{ //Data that will be saved in Redis.
		"refresh_token": refreshToken,
		"user_id":       user.UserID,
		"blacklisted":   false,
	}

	a.userRedis.InsertToken(context.Background(), sessionData, 14*24*time.Hour) //Saves the refresh token in Redis with an expiry of 14 days

	c.SetCookie("access_token", accessToken, 900, "/", "", true, true)           // 15 min
	c.SetCookie("refresh_token", refreshToken, 14*24*60*60, "/", "", true, true) // 14 days

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

	user := a.ur.GetAccountById(userID.(int))

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

// Checks whether the token is still valid and not blacklisted.
func introspect(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Placeholder for token introspection endpoint"})
}

// OIDC GET /authorize endpoint
// Checks whether the user is authenticated (and still has a valid tokenwa).
func authorize(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	responseType := c.Query("response_type")
	scope := c.Query("scope")
	state := c.Query("state")
	nonce := c.Query("nonce")

	if clientID == "" || redirectURI == "" || responseType != "code" || scope == "" || state == "" || nonce == "" {
		c.JSON(400, gin.H{"error": "Invalid Request"})
		return
	}

	loginURL := fmt.Sprintf("/login?client_id=%s&redirect_uri=%s&response_type=%s&scope=%s&state=%s&nonce=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(responseType),
		url.QueryEscape(scope),
		url.QueryEscape(state),
		url.QueryEscape(nonce),
	)

	//var userID string

	accessCookie, err := c.Cookie("access_token")
	if err != nil || accessCookie == "" {
		c.Redirect(302, loginURL)
		return
	}

	c.JSON(200, gin.H{"message": "Cookie works!"})
	fmt.Println(accessCookie)
}
