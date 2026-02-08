package main

import (
	"fmt"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	engine "raven/auth/DatabaseEngine"
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
		c.JSON(400, gin.H{"success": false, "reason": "Invalid email"})
		return
	}

	var user = engine.Users{}
	err = engine.GetQuery("SELECT * FROM users WHERE email = ?", &user, loginData.Email)

	result := auth.CheckPasswordHash(loginData.Password, user.Hashed_password)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "reason": err.Error()})
		return
	}

	if result {
		token, err := auth.GenerateToken()
		if err != nil {
			c.JSON(500, gin.H{"success": false, "reason": "Token generation failed."})
			return
		}

		expiredTime := time.Now().AddDate(0, 0, 180)
		affectedRows, err := engine.InsertQuery("INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)", *token, user.Id, time.Now().Format("2006-01-02 15:04:05"), expiredTime.Format("2006-01-02 15:04:05"))

		if affectedRows >= 1 { //Checks whether the amount of rows affected is greater / equal to 1.
			c.JSON(200, gin.H{"success": true, "token": token, "expiresAt": expiredTime.Unix()})
		}

	} else {
		c.JSON(401, gin.H{"success": false, "reason": "Invalid credentials"})
	}
}

func handleAbout(c *gin.Context) {
	c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", config.Version, config.Build)})
}
