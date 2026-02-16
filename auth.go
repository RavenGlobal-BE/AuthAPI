package main

import (
	"context"
	"fmt"
	config "raven/auth/Config"
	dbEngine "raven/auth/DatabaseEngine"
	logging "raven/auth/Logging"
	mailer "raven/auth/Mailer"

	"github.com/gin-gonic/gin"
)

type App struct {
	Ur *dbEngine.Usersrepo
	ms *mailer.MailService
}

func main() {
	logging.Log("Starting Raven Auth Server...", logging.Debug)

	gin.SetMode(gin.ReleaseMode)
	//r := gin.Default() //Debugging
	r := gin.New() //Production
	r.Use(gin.Recovery())

	db, err := dbEngine.NewDB(context.Background(), "postgres://postgres:6464@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		panic(err)
	}

	userRepo := dbEngine.NewUsersRepo(db)

	app := &App{
		Ur: userRepo,
		ms: mailer.NewSmtpService(),
	}

	RegisterRoutes(r, app)

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

func RegisterRoutes(router *gin.Engine, app *App) {
	router.POST("/login", handleLogin)
	router.GET("/about", handleAbout)
	router.GET("/dbTest", app.dbtest)
}
