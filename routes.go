package main

import (
	"fmt"
	auth "raven/auth/Authorization"
	engine "raven/auth/DatabaseEngine"
	config "raven/auth/config"
	"time"

	"github.com/gin-gonic/gin"
)

func handleLogin(c *gin.Context) {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := c.ShouldBindJSON(&loginData)

	mailValid := auth.EmailIsValid(loginData.Email)
	if mailValid == false {
		c.JSON(400, gin.H{"error": "Invalid email format"})
		return
	}

	var user = engine.Users{}
	err = engine.ExecuteQuery("SELECT * FROM users WHERE email = ?", &user, loginData.Email)

	result := auth.CheckPasswordHash(loginData.Password, user.Hashed_password)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
	} else {
		if result {
			token, err := auth.GenerateToken()
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to generate token. Try again later."})
				return
			}

			expiredTime := time.Now().AddDate(0, 0, 180).Format("2006-01-02 15:04:05")
			engine.ExecuteQueryInsert("INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)", *token, user.Id, time.Now().Format("2006-01-02 15:04:05"), expiredTime)

			c.JSON(200, gin.H{"Auth successful": loginData.Email, "Token": token, "Password Input": loginData.Password, "Hashed Password from DB": &user.Hashed_password})
		} else {
			c.JSON(401, gin.H{"Auth failed": loginData.Email})
		}
	}
}

func handleAbout(c *gin.Context) {
	c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", config.Version, config.Build)})
}
