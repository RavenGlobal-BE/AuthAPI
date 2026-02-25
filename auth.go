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
	ur        *dbEngine.Usersrepo
	ms        *mailer.MailService
	userRedis *dbEngine.TokenRepo
	rateLimit *dbEngine.RateRepo
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

	db, err := dbEngine.NewDB(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	defer db.Close() //Database closes when the server is shutting down.

	userRepo := dbEngine.NewUsersRepo(db)
	err = userRepo.SetupSchema()
	err = userRepo.SetupUsersTable()

	if err != nil {
		panic(err)
	}

	redisDataStore := dbEngine.CreateRedisClient(os.Getenv("RedisHost")+":"+os.Getenv("RedisPort"), os.Getenv("RedisPassword"), 0)
	userRedis := dbEngine.NewTokenRepo(redisDataStore)

	rateLimitRedis := dbEngine.CreateRedisClient(os.Getenv("RedisHost")+":"+os.Getenv("RedisPort"), os.Getenv("RedisPassword"), 1)
	rateRepo := dbEngine.NewRateRepo(rateLimitRedis)

	app := &App{
		ur:        userRepo,
		ms:        mailer.NewSmtpService(os.Getenv("MailUser"), os.Getenv("MailPassword"), os.Getenv("MailServer")),
		userRedis: userRedis,
		rateLimit: rateRepo,
	}

	RegisterRoutes(r, app)

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}

}

func RegisterRoutes(router *gin.Engine, app *App) {
	router.POST("/login", auth.RateLimiting(app.rateLimit), app.handleLogin)
	router.GET("/about", handleAbout)
	router.GET("/dbTest", auth.JWTAuthMiddleware(), app.dbtest)
	router.POST("/refresh", app.handleRefresh)
	router.POST("/introspect", introspect)
}
