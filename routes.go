package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	logger "raven/auth/Logging"
	mailer "raven/auth/Mailer"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var BuildDate string

// Returns both a Bearer token & a refresh token
func (a *App) handleLogin(c *gin.Context) {
	var loginData struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Nonce       string `json:"nonce"`
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		Language    string `json:"language"`

		DeviceModel string `json:"device_model"`
		DeviceName  string `json:"device_name"`
	}

	err := c.ShouldBindJSON(&loginData)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid request body"})
		return
	}

	setup := []string{}

	// Validate client_id exists and is active
	if loginData.ClientID == "" {
		c.JSON(400, gin.H{"success": false, "reason": "client_id is required"})
		return
	}

	mailValid := mailer.EmailIsValid(loginData.Email)
	if mailValid == false {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid email"})
		return
	}

	client := a.cr.GetClientByID(context.Background(), loginData.ClientID)
	if client == nil {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid client"})
		return
	}

	// Checks whether the redirection URI is correct
	if client.IsPublic == false && client.RedirectURI != loginData.RedirectURI {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid redirect URI"})
		return
	}

	// Checks whether the email actually exists
	user := a.ur.GetAccountByEmail(loginData.Email)
	if user == nil || user.IsDeleted == 1 {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		return
	}

	if user.CountryCode == nil || *user.CountryCode == "" {
		setup = append(setup, "countryCode")
	}

	if user.IsVerified == 0 {
		//SignUpInsert simulates a signup process to send the user their verification email.
		key, err := a.userRedis.SignUpInsert(context.Background(), user.Email)
		if err != nil {
			c.JSON(500, gin.H{"error": "Server error"})
			return
		}

		logger.Log("Key inserted: "+key, logger.Debug)

		logger.Log(fmt.Sprintf("%s/%s/verify?code=%s\n", os.Getenv("FRONTEND_URL"), loginData.Language, key), logger.Debug)
		a.mailService.AccountVerificationEmail(loginData.Email, user.FirstName, key, loginData.Language)
		c.JSON(401, gin.H{"success": false, "reason": "verification_pending"})
		return
	}

	//Backwards compatibility layer for users who are still on the old bcrypt system.
	if strings.HasPrefix(user.Password, "$2b$") || strings.HasPrefix(user.Password, "$2a$") || strings.HasPrefix(user.Password, "$2y$") {
		oldHash, err := auth.HashLegacyPassword(loginData.Password)
		if err != nil {
			logger.Log(err.Error(), logger.Error)
			c.JSON(500, gin.H{"success": false, "reason": "Server error"})
		}

		result := auth.CheckLegacyHash(oldHash, user.Password)
		if result == false {
			c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
		}

		newHash, err := auth.HashPassword(loginData.Password)
		err = a.ur.ResetPassword(user.Email, newHash)
		if err != nil {
			logger.Log(err.Error(), logger.Error)
			c.JSON(500, gin.H{"success": false, "reason": "Server error"})
		}
		return
	}

	// Else...
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

	refreshToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "refresh", time.Now().Add(14*24*time.Hour), loginData.Nonce, sessionID, *user.CountryCode, loginData.ClientID)
	accessToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "access", time.Now().Add(15*time.Minute), loginData.Nonce, sessionID, *user.CountryCode, loginData.ClientID)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": "Failed to generate tokens"})
		return
	}

	location, err := auth.GetIPLocation(c.ClientIP())
	if err != nil {
		logger.Log(err.Error(), logger.Error)
	}

	sessionData := map[string]interface{}{
		//Required info
		"user_id":     user.UserID,
		"blacklisted": false,
		"client_id":   loginData.ClientID,

		//Device information
		"device_model": loginData.DeviceModel,
		"device_name":  loginData.DeviceName,
		"language":     loginData.Language,

		"carrier": location["carrier"],
		"lat":     location["lat"],
		"lon":     location["lon"],
	}

	// Store session under raw sessionID — no hashing
	a.userRedis.InsertSession(context.Background(), sessionID, user.UserID, sessionData, 14*24*time.Hour)

	/* Removed token setters in cookies as it's no longer required. How they're still here in case somebody else needs it. */
	//c.SetCookie("access_token", accessToken, 900, "/", "", false, true)           // 15 min
	//c.SetCookie("refresh_token", refreshToken, 14*24*60*60, "/", "", false, true) // 14 days

	if len(setup) >= 1 {
		c.SetCookie("setup_session", sessionID, 3600, "/", "", false, true)
	}

	c.JSON(200, gin.H{
		"success":       true,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"additional":    setup,
	})
}

func (a *App) verify(c *gin.Context) {
	verificationlink := c.Query("id")

	if verificationlink == "" {
		c.JSON(400, gin.H{"success": false, "reason": "No link passed"})
		return
	}

	data, err := a.userRedis.VerifySignup(context.Background(), verificationlink)
	if err != nil {
		c.JSON(400, gin.H{"success": false, "reason": "Invalid link"})
		return
	}

	err = a.ur.VerifyAccount(data["email"])
	if err != nil {
		logger.Log(err.Error(), logger.Error)
		c.JSON(400, gin.H{"success": false, "reason": "Failed to verify account"})
		return
	}

	c.JSON(200, gin.H{"success": true})
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
	c.JSON(200, gin.H{
		"version":    config.Version,
		"build":      config.Build,
		"build_date": BuildDate,
	})
}

// Checks whether the token is still valid and not blacklisted. Only use it on sensitive operations
func (a *App) introspect(c *gin.Context) {
	var data struct {
		AccessToken string `json:"token"`
	}

	err := c.ShouldBindJSON(&data)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
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

	refreshToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "refresh", time.Now().Add(14*24*time.Hour), nonce, tokenSessionID, *user.CountryCode, clientID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}
	accessToken, err := auth.GenerateJWTToken(user.UserID, user.Email, user.FirstName, user.LastName, "access", time.Now().Add(15*time.Minute), nonce, tokenSessionID, *user.CountryCode, clientID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	a.userRedis.InsertSession(context.Background(), tokenSessionID, user.UserID, map[string]interface{}{
		"user_id": user.UserID, "blacklisted": false,
	}, 14*24*time.Hour)

	c.JSON(200, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    900,
	})
}

type JWTData struct {
	Email     string  `json:"email"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Country   *string `json:"country"`
	Username  string  `json:"username"`
}

func (a *App) refresh(c *gin.Context) {
	var req struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		UpdateInfo   bool   `json:"update_info"` //Re-fetches the user in question to update the token with the latest info.
	}

	userData := new(JWTData)

	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken)
	if err != nil || claims.TokenType != "refresh" {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}

	userData.Email = claims.Email
	userData.FirstName = claims.FirstName
	userData.LastName = claims.LastName
	*userData.Country = claims.Country
	userData.Username = claims.Username

	// Use sessionID directly from refresh token claims
	tokenData, redisErr := a.userRedis.GetSessionByID(context.Background(), claims.SessionID)
	if redisErr != nil || tokenData["blacklisted"] == "1" {
		c.JSON(401, gin.H{"error": "Invalid token"})
		return
	}

	parsedID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	if req.UpdateInfo {
		updatedInfo := a.ur.GetAccountById(int(parsedID))
		if updatedInfo != nil {
			userData.LastName = updatedInfo.LastName
			userData.FirstName = updatedInfo.FirstName
			userData.Email = updatedInfo.Email
			userData.Country = updatedInfo.CountryCode
		}
	}

	newAccessToken, err := auth.GenerateJWTToken(parsedID, userData.Email, userData.FirstName, claims.LastName, "access", time.Now().Add(15*time.Minute), claims.Nonce, claims.SessionID, *userData.Country, claims.Audience[0])
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	response := gin.H{"access_token": newAccessToken, "token_type": "Bearer", "expires_in": 900}

	// Sliding window: rotate refresh token only when < 2 days remain
	if time.Until(claims.ExpiresAt.Time) < 2*24*time.Hour {
		newSidPtr, newSidErr := auth.GenerateToken()
		if newSidErr == nil {
			newSessionID := *newSidPtr
			newRefreshToken, err := auth.GenerateJWTToken(parsedID, claims.Email, claims.FirstName, claims.LastName, "refresh", time.Now().Add(14*24*time.Hour), claims.Nonce, newSessionID, *userData.Country, claims.Audience[0])
			if err == nil {
				newAccessToken, _ = auth.GenerateJWTToken(parsedID, claims.Email, claims.FirstName, claims.LastName, "access", time.Now().Add(15*time.Minute), claims.Nonce, newSessionID, *userData.Country, claims.Audience[0])
				a.userRedis.DeleteToken(context.Background(), claims.SessionID)

				a.userRedis.InsertSession(context.Background(), newSessionID, parsedID, map[string]interface{}{
					"user_id": parsedID, "blacklisted": false, "client_id": tokenData["client_id"],
				}, 14*24*time.Hour)
				response["access_token"] = newAccessToken
				response["refresh_token"] = newRefreshToken
			}
		}
	} else {
		sessionKey := fmt.Sprintf("session:%d:%s", parsedID, claims.SessionID)
		a.userRedis.ExtendSession(context.Background(), sessionKey)
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
		Language    string `json:"language"` //This is a 2-letter code
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	hashedPass, err := auth.HashPassword(req.Password)

	err = a.ur.RegisterAccount(req.Email, hashedPass, req.FirstName, req.LastName, req.CountryCode, req.Username)
	if err != nil {
		fmt.Println(err)
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	key, err := a.userRedis.SignUpInsert(context.Background(), req.Email)
	if err != nil {
		fmt.Println(err)
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	logger.Log("Key inserted: "+key, logger.Debug)

	logger.Log(fmt.Sprintf("%s/%s/verify?code=%s\n", os.Getenv("FRONTEND_URL"), req.Language, key), logger.Debug)
	a.mailService.AccountVerificationEmail(req.Email, req.FirstName, key, req.Language)
	c.JSON(200, gin.H{"message": "Registered successfully."})
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

// Resets the user's password (ONLY BE RAN AFTER THE TOKEN IS GUARENTEED TO BE AVAILABLE)
func (a *App) resetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	if req.Token == "" || req.Password == "" {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	data, err := a.userRedis.VerifyReset(context.Background(), req.Token)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	hashedPass, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	call := a.ur.ResetPassword(data["email"], hashedPass)
	if call != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	c.JSON(200, gin.H{"message": "Password reset successfully."})
}

// Sends an email to the user to reset their password
func (a *App) requestReset(c *gin.Context) {
	mail := c.Query("email")
	language := c.Query("lang")
	if !mailer.EmailIsValid(mail) {
		c.JSON(400, gin.H{"error": "Invalid email"})
		return
	}

	user := a.ur.GetAccountByEmail(mail)
	if user == nil { //If the user isn't found, we don't want to tell them that, so we just return
		c.JSON(200, gin.H{"message": "Reset email sent successfully."})
		return
	}

	key, err := a.userRedis.ResetInsert(context.Background(), mail)
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}

	logger.Log("Key inserted: "+key, logger.Debug)

	a.mailService.ResetPasswordEmail(mail, user.FirstName, key, language)
	c.JSON(200, gin.H{"message": "Reset email sent successfully."})
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

	loginURL := fmt.Sprintf("http://localhost:3001/fr?client_id=%s&redirect_uri=%s&response_type=%s&scope=%s&state=%s&nonce=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(responseType),
		url.QueryEscape(scope),
		url.QueryEscape(state),
		url.QueryEscape(nonce),
	)

	accessCookie, _ := c.Cookie("access_token")
	refreshCookie, _ := c.Cookie("refresh_token")

	// If neither cookie exists, send to "login" immediately
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
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.Redirect(302, loginURL)
			return
		}

		refreshClaims, refreshErr := auth.ValidateToken(refreshCookie)
		if refreshErr != nil || refreshClaims.TokenType != "refresh" {
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.SetCookie("refresh_token", "", -1, "/", "", false, true)
			c.Redirect(302, loginURL)
			return
		}

		// Check Redis: token must exist and not be blacklisted
		tokenData, redisErr := a.userRedis.GetTokenInfo(context.Background(), refreshCookie)
		if redisErr != nil || len(tokenData) == 0 || tokenData["blacklisted"] == "1" {
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.SetCookie("refresh_token", "", -1, "/", "", false, true)
			c.Redirect(302, loginURL)
			return
		}

		// Re-issue the access token silently
		parsedID, parseErr := strconv.ParseInt(refreshClaims.Subject, 10, 64)
		if parseErr != nil {
			c.JSON(500, gin.H{"error": "Server error"})
			return
		}

		newAccessToken, tokenErr := auth.GenerateJWTToken(
			parsedID,
			refreshClaims.Email,
			refreshClaims.FirstName,
			refreshClaims.LastName,
			"access",
			time.Now().Add(15*time.Minute),
			nonce,
			refreshClaims.SessionID,
			refreshClaims.Country,
			refreshClaims.Audience[0],
		)
		if tokenErr != nil {
			c.JSON(500, gin.H{"error": "Server error"})
			return
		}

		c.SetCookie("access_token", newAccessToken, 900, "/", "", false, true)
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

func (a *App) setCountryCode(c *gin.Context) {
	userID, exists := c.Get("user_id")

	if exists != true {
		c.JSON(400, gin.H{"error": "Invalid token"})
		return
	}

	err := a.ur.SetCountryCode(userID.(int64), c.Query("set"))
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to set country code"})
		return
	}

	c.JSON(200, gin.H{"success": false})
}

func (a *App) devices(c *gin.Context) {
	userID, exists := c.Get("user_id")

	if exists != true {
		c.JSON(400, gin.H{"error": "Invalid token"})
		return
	}

	devices, err := a.userRedis.GetDevices(context.Background(), userID.(int))
	if err != nil {
		c.JSON(500, gin.H{"error": "Server error"})
		return
	}
	c.JSON(200, gin.H{"devices": devices})
}

var tokenBlacklistedError = errors.New("Token blacklisted")
var clientNotFoundError = errors.New("Client ID does not exist")

func (a *App) introspectToken(AccessToken string) (bool, error) {
	claims, err := auth.ValidateToken(AccessToken)
	if err != nil {
		return false, err
	}

	logger.Log(claims.SessionID, logger.Debug)

	session, sessionErr := a.userRedis.GetSessionByID(context.Background(), claims.SessionID)
	if sessionErr != nil || session["blacklisted"] == "1" {
		return false, tokenBlacklistedError
	}

	client := a.cr.GetClientByID(context.Background(), claims.Audience[0])
	if client == nil {
		return false, clientNotFoundError
	}

	return true, nil
}
