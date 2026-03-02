package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	logger "raven/auth/Logging"
	mailer "raven/auth/Mailer"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Returns both a Bearer token & a refresh token
func (a *App) handleLogin(c *gin.Context) {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nonce    string `json:"nonce"`
		ClientID string `json:"client_id"`
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

	// Validate client_id exists and is active
	if loginData.ClientID == "" {
		c.JSON(400, gin.H{"success": false, "reason": "client_id is required"})
		return
	}
	client := a.cr.GetClientByID(context.Background(), loginData.ClientID)
	if client == nil {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid client"})
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

	// Generate sessionID first — embedded in both tokens, used as the Redis key
	sidPtr, sidErr := auth.GenerateToken()
	if sidErr != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate session"})
		return
	}
	sessionID := *sidPtr

	refreshToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "refresh", time.Now().Add(14*24*time.Hour), loginData.Nonce, sessionID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate tokens"})
		return
	}
	accessToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "access", time.Now().Add(15*time.Minute), loginData.Nonce, sessionID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate tokens"})
		return
	}

	sessionData := map[string]interface{}{
		"user_id":     user.UserID,
		"blacklisted": false,
		"client_id":   loginData.ClientID,
	}

	// Store session under raw sessionID — no hashing
	a.userRedis.InsertSession(context.Background(), sessionID, sessionData, 14*24*time.Hour)

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
func (a *App) introspect(c *gin.Context) {
	var data struct {
		AccessToken string `json:"token"`
	}

	err := c.ShouldBindJSON(&data)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	claims, err := auth.ValidateToken(data.AccessToken)
	if err != nil {
		c.JSON(200, gin.H{"active": false})
		return
	}

	logger.Log(claims.SessionID, logger.Debug)

	session, sessionErr := a.userRedis.GetSessionByID(context.Background(), claims.SessionID)
	if sessionErr != nil || session["blacklisted"] == "1" {
		c.JSON(200, gin.H{"active": false})
		return
	}
	c.JSON(200, gin.H{
		"active":     true,
		"sub":        claims.Subject,
		"exp":        claims.ExpiresAt.Unix(),
		"token_type": claims.TokenType,
	})

}

func (a *App) token(c *gin.Context) {
	code := c.PostForm("code")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")
	redirectURI := c.PostForm("redirect_uri")

	if code == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// Look up the client in the DB
	client := a.cr.GetClientByID(context.Background(), clientID)
	if client == nil {
		c.JSON(401, gin.H{"error": "Invalid client"})
		return
	}

	// Verify the client secret against the stored hash
	if client.IsPublic {
		// Public client: verify PKCE code_verifier
		codeVerifier := c.PostForm("code_verifier")
		if codeVerifier == "" {
			c.JSON(401, gin.H{"error": "code_verifier required for public clients"})
			return
		}
		// Will be verified against stored code_challenge after consuming the auth code
	} else {
		// Confidential client: verify client_secret
		if !auth.CheckPasswordHash(clientSecret, client.ClientSecret) {
			c.JSON(401, gin.H{"error": "Invalid client"})
			return
		}
	}

	// Validate redirect_uri against what's registered for this client
	if client.RedirectURI != redirectURI {
		c.JSON(401, gin.H{"error": "Invalid client"})
		return
	}

	// Consume the auth code — reads + deletes from Redis (one-time use)
	authData, err := a.userRedis.GetAuthCode(context.Background(), code)
	if err != nil {
		c.JSON(401, gin.H{"error": "Invalid grant"})
		return
	}

	// Validate client_id and redirect_uri match what was stored during /authorize
	if authData["client_id"] != clientID || authData["redirect_uri"] != redirectURI {
		c.JSON(401, gin.H{"error": "Invalid grant"})
		return
	}

	// For public clients, verify PKCE: SHA256(code_verifier) must match stored code_challenge
	if client.IsPublic {
		codeVerifier := c.PostForm("code_verifier")
		hash := sha256.Sum256([]byte(codeVerifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		if computed != authData["code_challenge"] {
			c.JSON(401, gin.H{"error": "Invalid grant"})
			return
		}
	}

	// Look up the user to get their details for the JWT
	userIDInt, parseErr := strconv.Atoi(authData["user_id"])
	if parseErr != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	user := a.ur.GetAccountById(userIDInt)
	if user == nil {
		c.JSON(401, gin.H{"error": "Invalid grant"})
		return
	}

	nonce := authData["nonce"]

	// Generate sessionID independently — embedded in both tokens
	tokenSidPtr, tokenSidErr := auth.GenerateToken()
	if tokenSidErr != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}
	tokenSessionID := *tokenSidPtr

	refreshToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "refresh", time.Now().Add(14*24*time.Hour), nonce, tokenSessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}
	accessToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "access", time.Now().Add(15*time.Minute), nonce, tokenSessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	a.userRedis.InsertSession(context.Background(), tokenSessionID, map[string]interface{}{
		"user_id": user.UserID, "blacklisted": false,
	}, 14*24*time.Hour)

	c.JSON(200, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    900,
	})
}

func (a *App) refresh(c *gin.Context) {
	var req struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken)
	if err != nil || claims.TokenType != "refresh" {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}

	// Use sessionID directly from refresh token claims
	tokenData, redisErr := a.userRedis.GetSessionByID(context.Background(), claims.SessionID)
	if redisErr != nil || tokenData["blacklisted"] == "1" {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}

	parsedID, err := strconv.ParseInt(claims.Subject, 10, 32)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	newAccessToken, err := auth.GenerateJWTToken(int32(parsedID), claims.Email, claims.FirstName, claims.LastName, "access", time.Now().Add(15*time.Minute), claims.Nonce, claims.SessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	response := gin.H{"access_token": newAccessToken, "token_type": "Bearer", "expires_in": 900}

	// Sliding window: rotate refresh token only when < 4 days remain (i.e. 10+ days old)
	if time.Until(claims.ExpiresAt.Time) < 4*24*time.Hour {
		newSidPtr, newSidErr := auth.GenerateToken()
		if newSidErr == nil {
			newSessionID := *newSidPtr
			newRefreshToken, err := auth.GenerateJWTToken(int32(parsedID), claims.Email, claims.FirstName, claims.LastName, "refresh", time.Now().Add(14*24*time.Hour), claims.Nonce, newSessionID)
			if err == nil {
				newAccessToken, _ = auth.GenerateJWTToken(int32(parsedID), claims.Email, claims.FirstName, claims.LastName, "access", time.Now().Add(15*time.Minute), claims.Nonce, newSessionID)
				a.userRedis.DeleteToken(context.Background(), claims.SessionID)
				a.userRedis.InsertSession(context.Background(), newSessionID, map[string]interface{}{
					"user_id": int32(parsedID), "blacklisted": false, "client_id": tokenData["client_id"],
				}, 14*24*time.Hour)
				response["access_token"] = newAccessToken
				response["refresh_token"] = newRefreshToken
			}
		}
	}

	c.JSON(200, response)
}

func (a *App) register(c *gin.Context) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Username    string `json:"username"`
		CountryCode string `json:"country_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	c.JSON(200, gin.H{"message": "Register"})
}

func (a *App) logout(c *gin.Context) {
	accessToken := c.GetHeader("Authorization")
	if !strings.HasPrefix(accessToken, "Bearer ") {
		c.JSON(401, gin.H{"error": "Unformatted token"})
		return
	}

	token := strings.Split(accessToken, "Bearer ")[1]

	claims, err := auth.ValidateToken(token)
	if err != nil || claims.TokenType != "refresh" {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}

	a.userRedis.DeleteToken(context.Background(), claims.SessionID)
	c.JSON(200, gin.H{"message": "Logged out successfully"})
}

// OIDC GET /authorize endpoint
// Checks whether the user is authenticated (and still has a valid token).
func (a *App) authorize(c *gin.Context) {
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

	// Validate the client_id against registered apps
	client := a.cr.GetClientByID(context.Background(), clientID)
	if client == nil {
		c.JSON(400, gin.H{"error": "invalid_client"})
		return
	}

	// Also ensure the redirect_uri matches the registered one
	if client.RedirectURI != redirectURI {
		c.JSON(400, gin.H{"error": "invalid_redirect_uri"})
		return
	}

	loginURL := fmt.Sprintf("https://auth.raven.co.com/login?client_id=%s&redirect_uri=%s&response_type=%s&scope=%s&state=%s&nonce=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(responseType),
		url.QueryEscape(scope),
		url.QueryEscape(state),
		url.QueryEscape(nonce),
	)

	accessCookie, _ := c.Cookie("access_token")
	refreshCookie, _ := c.Cookie("refresh_token")

	// If neither cookie exists, send to login immediately
	if accessCookie == "" && refreshCookie == "" {
		c.Redirect(302, loginURL)
		return
	}

	var userID string

	// Validate the access token
	cookieDetails, err := auth.ValidateToken(accessCookie)
	if err != nil || cookieDetails.ExpiresAt.Before(time.Now()) || cookieDetails.TokenType != "access" {
		// Access token invalid or expired — try the refresh token
		if refreshCookie == "" {
			c.SetCookie("access_token", "", -1, "/", "", true, true)
			c.Redirect(302, loginURL)
			return
		}

		refreshClaims, refreshErr := auth.ValidateToken(refreshCookie)
		if refreshErr != nil || refreshClaims.TokenType != "refresh" {
			c.SetCookie("access_token", "", -1, "/", "", true, true)
			c.SetCookie("refresh_token", "", -1, "/", "", true, true)
			c.Redirect(302, loginURL)
			return
		}

		// Check Redis: token must exist and not be blacklisted
		tokenData, redisErr := a.userRedis.GetTokenInfo(context.Background(), refreshCookie)
		if redisErr != nil || len(tokenData) == 0 || tokenData["blacklisted"] == "1" {
			c.SetCookie("access_token", "", -1, "/", "", true, true)
			c.SetCookie("refresh_token", "", -1, "/", "", true, true)
			c.Redirect(302, loginURL)
			return
		}

		// Re-issue the access token silently
		parsedID, parseErr := strconv.ParseInt(refreshClaims.Subject, 10, 32)
		if parseErr != nil {
			c.JSON(500, gin.H{"error": "Server error"})
			return
		}

		newAccessToken, tokenErr := auth.GenerateJWTToken(
			int32(parsedID),
			refreshClaims.Email,
			refreshClaims.FirstName,
			refreshClaims.LastName,
			"access",
			time.Now().Add(15*time.Minute),
			nonce,
			refreshClaims.SessionID,
		)
		if tokenErr != nil {
			c.JSON(500, gin.H{"error": "Server error"})
			return
		}

		c.SetCookie("access_token", newAccessToken, 900, "/", "", true, true)
		userID = tokenData["user_id"]
	} else {
		userID = cookieDetails.Subject
	}

	// Generate auth code and store in Redis with 60s TTL
	code, codeErr := auth.GenerateToken()
	if codeErr != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	authCodeData := map[string]interface{}{
		"user_id":      userID,
		"client_id":    clientID,
		"redirect_uri": redirectURI,
		"nonce":        nonce,
		"scope":        scope,
	}

	// For public clients (mobile/SPA), require and store the PKCE code_challenge
	if client.IsPublic {
		codeChallenge := c.Query("code_challenge")
		codeChallengeMethod := c.Query("code_challenge_method")
		if codeChallenge == "" || codeChallengeMethod != "S256" {
			c.JSON(400, gin.H{"error": "PKCE required for this client"})
			return
		}
		authCodeData["code_challenge"] = codeChallenge
	}

	if err := a.userRedis.InsertAuthCode(context.Background(), *code, authCodeData); err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	// Redirect back to the third-party app with the auth code
	redirect := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, url.QueryEscape(*code), url.QueryEscape(state))
	c.Redirect(302, redirect)
}
