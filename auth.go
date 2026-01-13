package main

import (
	"fmt"
	auth "raven/auth/Authorization"
	engine "raven/auth/DatabaseEngine" // Database Engine Module

	"github.com/gin-gonic/gin" //Gin Web Framework
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())

	engine.ConnectDB() //Pings the DB in advance

	r.POST("/login", func(c *gin.Context) {
		var loginData struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		err := c.ShouldBindJSON(&loginData) //Bind the received JSON to the loginData struct

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

				c.JSON(200, gin.H{"Auth successful": loginData.Email, "Token": token, "Password Input": loginData.Password, "Hashed Password from DB": &user.Hashed_password})
			} else {
				c.JSON(401, gin.H{"Auth failed": loginData.Email})
			}
		}
	})

	r.GET("/about", func(c *gin.Context) {
		c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", version, build)})
	})

	fmt.Printf("Webserver started. (Port %d)", port)
	r.Run(fmt.Sprintf(":%d", port))
}
