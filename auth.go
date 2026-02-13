package main

import (
	"fmt"
	config "raven/auth/Config"
	logging "raven/auth/Logging"

	"github.com/gin-gonic/gin"
)

func main() {
	logging.Log("Starting Raven Auth Server...", logging.Debug)

	gin.SetMode(gin.ReleaseMode)
	//r := gin.Default() //Debugging
	r := gin.New() //Production
	r.Use(gin.Recovery())

	logging.Log("Connecting to server...", logging.Debug)
	//engine.ConnectDB("Accounts")

	RegisterRoutes(r)
	//mailer.Send()

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

func RegisterRoutes(router *gin.Engine) {
	router.POST("/login", handleLogin)
	router.GET("/about", handleAbout)
	router.GET("/dbTest", dbtest)
}
