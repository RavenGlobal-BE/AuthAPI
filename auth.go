package main

import (
	"context"
	"fmt"
	config "raven/auth/Config"
	dbEngine "raven/auth/DatabaseEngine"
	logging "raven/auth/Logging"
	mailer "raven/auth/Mailer"

	"os"

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

	dbURL := os.Getenv("DATABASE_URL")

	db, err := dbEngine.NewDB(context.Background(), dbURL)
	if err != nil {
		panic(err)
	}

	userRepo := dbEngine.NewUsersRepo(db)
	err = userRepo.SetupSchema()
	if err != nil {
		panic(err)
	}
	err = userRepo.SetupUsersTable()
	if err != nil {
		panic(err)
	}

	app := &App{
		Ur: userRepo,
		ms: mailer.NewSmtpService(os.Getenv("MailUser"), os.Getenv("MailPassword"), os.Getenv("MailServer")),
	}

	RegisterRoutes(r, app)

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

func RegisterRoutes(router *gin.Engine, app *App) {
	router.POST("/login", app.handleLogin)
	router.GET("/about", handleAbout)
	router.GET("/dbTest", app.dbtest)
}
