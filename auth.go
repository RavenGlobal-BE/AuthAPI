package main

import (
	"context"
	"fmt"
	config "raven/auth/Config"
	dbEngine "raven/auth/DatabaseEngine"
	logging "raven/auth/Logging"

	"github.com/gin-gonic/gin"
)

type App struct {
	Ur *dbEngine.Usersrepo
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
	}

	RegisterRoutes(r, app)
	//mailer.Send()

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
