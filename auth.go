package main

import (
	"context"
	"fmt"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	dbEngine "raven/auth/DatabaseEngine"
	logging "raven/auth/Logging"
	mailer "raven/auth/Mailer"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type App struct {
	Ur *dbEngine.Usersrepo
	ms *mailer.MailService
}

func main() {
	logging.Log("Starting Raven Auth Server...", logging.Debug)

	//generateKeyPair()

	if err := godotenv.Load(); err != nil {
		logging.Log("Warning: .env file not found", logging.Warning)
	}

	gin.SetMode(gin.ReleaseMode)
	//r := gin.Default() //Debugging
	r := gin.New() //Production
	r.Use(gin.Recovery())

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:6464@localhost:5432/prod?sslmode=disable"
	}

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

	db.Close() //Database closes when the server is shutting down.
}

func RegisterRoutes(router *gin.Engine, app *App) {
	router.POST("/login", app.handleLogin)
	router.GET("/about", handleAbout)
	router.GET("/dbTest", auth.JWTAuthMiddleware(), app.dbtest)
}
