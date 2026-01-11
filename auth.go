package main

import (
	"fmt"
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

		err := c.ShouldBindJSON(&loginData)

		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		}

		c.JSON(200, gin.H{"E-Mail Input": loginData.Email, "Password Input": loginData.Password})
	})

	r.GET("/about", func(c *gin.Context) {
		c.JSON(200, gin.H{"about": fmt.Sprintf("v%s (build %.1f)", version, build)})
	})

	fmt.Println("\033[33;1mWebserver started \033[0m")
	r.Run()
}
