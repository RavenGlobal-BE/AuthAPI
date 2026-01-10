package main

import (
	"fmt"
	"net/http"
	authorization "raven/auth/Authorization" // Authorization Module

	"github.com/gin-gonic/gin" //Gin Web Framework
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())

	authorization.DbTest()

	r.GET("/login", func(c *gin.Context) {
		password := c.Query("passtest")
		hash, err := authorization.HashPassword(password)
		if err != nil {

		}

		match := authorization.CheckPasswordHash(password, hash)
		fmt.Println("Match:   ", match)

		c.JSON(http.StatusOK, gin.H{
			"encryptedPass": hash,
		})
	})

	fmt.Println("\033[33;1mWebserver started \033[0m")
	r.Run()
}
