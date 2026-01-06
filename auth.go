package main

import (
	"fmt"
	"net/http"
	authorization "raven/auth/Authorization"

	"github.com/gin-gonic/gin" //Gin Web Framework
)

func main() {
	var auth authorization.Authorization
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())

	r.GET("/ping", func(c *gin.Context) {
		password := c.Query("passtest")
		hash, err := auth.HashPassword(password)
		if err != nil {

		}

		match := auth.CheckPasswordHash(password, hash)
		fmt.Println("Match:   ", match)

		c.JSON(http.StatusOK, gin.H{
			"encryptedPass": hash,
		})
	})

	fmt.Println("\033[33;1mWebserver started \033[0m")
	r.Run()
}
