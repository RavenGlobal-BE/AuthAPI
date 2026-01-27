package main

import (
	"fmt"
	engine "raven/auth/DatabaseEngine"
	config "raven/auth/config"
	logging "raven/auth/logging"

	"github.com/gin-gonic/gin"
)

func main() {
	logging.Log("Starting Raven Auth Server...", logging.Debug)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())

	logging.Log("Connecting to server...", logging.Debug)
	engine.ConnectDB("Accounts")

	RegisterRoutes(r)

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

func RegisterRoutes(r *gin.Engine) {
	r.POST("/login", handleLogin)
	r.GET("/about", handleAbout)
}
