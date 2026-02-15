package main

import (
	"fmt"

	//auth "raven/auth/Authorization"
	config "raven/auth/Config"
	engine "raven/auth/DatabaseEngine"

	//mailer "raven/auth/Mailer"

	//"time"

	"github.com/gin-gonic/gin"
)

// Returns both a Bearer token & a refresh token
func handleLogin(c *gin.Context) {
	// var loginData struct {
	// 	Email    string `json:"email"`
	// 	Password string `json:"password"`
	// }

	// err := c.ShouldBindJSON(&loginData) //Pointer value

	// mailValid := mailer.EmailIsValid(loginData.Email)
	// if mailValid == false {
	// 	c.JSON(400, gin.H{"success": false, "reason": "Invalid email"})
	// 	return
	// }

	// var user = engine.Users{}
	// err = engine.GetQuery("SELECT * FROM users WHERE email = ?", &user, loginData.Email)

	// result := auth.CheckPasswordHash(loginData.Password, user.Hashed_password)

	// if err != nil {
	// 	c.JSON(500, gin.H{"reason": err.Error()})
	// 	return
	// }

	// if result {
	// 	token, err := auth.GenerateToken() //Generates a bearer token
	// 	if err != nil {
	// 		c.JSON(500, gin.H{"reason": "Internal error"})
	// 		return
	// 	}

	// 	//refresh, err := auth.GenerateToken() //Generates a refresh token

	// 	expiredTime := time.Now().AddDate(0, 0, 1) // 24 hours
	// 	affectedRows, err := engine.InsertQuery("INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)", *token, user.Id, time.Now().Format("2006-01-02 15:04:05"), expiredTime.Format("2006-01-02 15:04:05"))

	// 	if affectedRows >= 1 { //Checks whether the amount of rows affected is greater / equal to 1.
	// 		c.JSON(200, gin.H{"token": token, "expiresAt": expiredTime.Unix(), "Refresh": "soon"})
	// 		return
	// 	}

	// 	c.JSON(500, gin.H{"reason": "Internal error"})

	// } else {
	// 	c.JSON(401, gin.H{"reason": "Invalid credentials"})
	// }
}

// Shows the user the current version of the API
func handleAbout(c *gin.Context) {
	c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", config.Version, config.Build)})
}

func dbtest(c *gin.Context) {
	mob := engine.MobileInfo{}
	//	engine.PGQuery(context.Background(), &mob)

	c.JSON(200, gin.H{"mobileInfo": mob})
}
