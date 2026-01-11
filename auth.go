package main

import (
	"fmt"                              // Authorization Module
	engine "raven/auth/DatabaseEngine" // Database Engine Module

	"github.com/gin-gonic/gin" //Gin Web Framework
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())

	engine.ConnectDB()

	r.GET("/login", func(c *gin.Context) {
		password := c.Query("passtest")

		c.JSON(200, gin.H{"PasswordInput": password})
	})

	fmt.Println("\033[33;1mWebserver started \033[0m")
	r.Run()
}
