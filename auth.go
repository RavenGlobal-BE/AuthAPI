package main

import (
	"context"
	"fmt"
	auth "raven/auth/Authorization"
	config "raven/auth/Config"
	dbEngine "raven/auth/DatabaseEngine"
	logging "raven/auth/Logging"
	mailer "raven/auth/Mailer"
	"strings"
	"time"

	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

/* SUPPORT FOR PASSKEYS & MFA ARE COMING IN AUTH v2.1 */

type App struct {
	ur        *dbEngine.Usersrepo
	ms        *mailer.MailService
	userRedis *dbEngine.TokenRepo
	rateLimit *dbEngine.RateRepo
	cr        *dbEngine.ClientsRepo
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
	for {
		err = userRepo.Init()
		if err == nil { //connected Successfully
			break
		}
		if !strings.Contains(err.Error(), "57P03") {
			panic(err) // real error, not a startup race
		}
		logging.Log("Postgres is starting up, retrying in 15 seconds...", logging.Warning)
		time.Sleep(15 * time.Second)
	}

	redisDataStore := dbEngine.CreateRedisClient(os.Getenv("RedisHost")+":"+os.Getenv("RedisPort"), os.Getenv("RedisPassword"), 0)
	userRedis := dbEngine.NewTokenRepo(redisDataStore)

	rateLimitRedis := dbEngine.CreateRedisClient(os.Getenv("RedisHost")+":"+os.Getenv("RedisPort"), os.Getenv("RedisPassword"), 1)
	rateRepo := dbEngine.NewRateRepo(rateLimitRedis)

	clientsRepo := dbEngine.NewClientsRepo(db)

	app := &App{
		ur:        userRepo,
		ms:        mailer.NewSmtpService(os.Getenv("MailUser"), os.Getenv("MailPassword"), os.Getenv("MailServer")),
		userRedis: userRedis,
		rateLimit: rateRepo,
		cr:        clientsRepo,
	}

	RegisterRoutes(r, app)

	logging.Log(fmt.Sprintf("Starting Raven ONE Auth on port %d...", config.Port), logging.Info)
	if err := r.Run(fmt.Sprintf(":%d", config.Port)); err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

func RegisterRoutes(router *gin.Engine, app *App) {
	//Misc
	router.GET("/about", handleAbout)
	router.GET("/dbTest", auth.JWTAuthMiddleware(), app.dbtest) //Used as test

	//User actions
	router.POST("/login", auth.RateLimiting(app.rateLimit), app.handleLogin)
	router.POST("/register", auth.RateLimiting(app.rateLimit), app.register)

	//Token checker
	router.POST("/introspect", app.introspect)
	router.GET("/authorize", app.authorize)

	//Token management
	router.POST("/token", auth.RateLimiting(app.rateLimit), app.token)
	router.POST("/refresh", auth.RateLimiting(app.rateLimit), app.refresh)
	router.POST("/logout", auth.RateLimiting(app.rateLimit), app.logout)

	//Well known routes
	router.GET("/.well-known/jwks.json", JWTKeys)
	router.GET("/.well-known/openid-configuration", OpenIDConfig)

	//Personalization
	router.GET("/todayBG", app.todayBG)
	router.GET("/clientInfo", app.clientInfo)
}
