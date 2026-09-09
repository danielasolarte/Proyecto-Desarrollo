package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"

	adminHTTP "github.com/equipo-mooc/plataforma-mooc/internal/admin/http"
	authHTTP "github.com/equipo-mooc/plataforma-mooc/internal/auth/http"
	authPG "github.com/equipo-mooc/plataforma-mooc/internal/auth/postgres"
	catalogHTTP "github.com/equipo-mooc/plataforma-mooc/internal/catalog/http"
	catalogPG "github.com/equipo-mooc/plataforma-mooc/internal/catalog/postgres"
	coursesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/courses/http"
	coursesPG "github.com/equipo-mooc/plataforma-mooc/internal/courses/postgres"
	quizzesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/http"
	quizzesPG "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/postgres"
	quizzesUC "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/mailer"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable"
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}
	defer db.Close()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	smtpAddr := os.Getenv("SMTP_ADDR")
	if smtpAddr == "" {
		smtpAddr = "localhost:1025"
	}
	mailFrom := os.Getenv("MAIL_FROM")
	if mailFrom == "" {
		mailFrom = "no-reply@mooc.local"
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	userRepo := authPG.NewUserRepository(db)
	sessionStore := authPG.NewRedisSessionStore(redisClient)
	mailerClient := mailer.NewSMTPMailer(smtpAddr, mailFrom)
	e.Use(authHTTP.AuthMiddleware(userRepo, sessionStore))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	courseRepo := coursesPG.NewCourseRepository(db)
	courseHandler := coursesHTTP.NewHandler(courseRepo)
	coursesHTTP.RegisterRoutes(e, courseHandler)

	authHandler := authHTTP.NewHandler(userRepo, sessionStore, mailerClient)
	authHTTP.RegisterRoutes(e, authHandler)

	adminHandler := adminHTTP.NewHandler(userRepo, sessionStore)
	adminHTTP.RegisterRoutes(e, adminHandler)

	catalogRepo := catalogPG.NewCatalogRepository(db)
	catalogHandler := catalogHTTP.NewHandler(catalogRepo)
	catalogHTTP.RegisterRoutes(e, catalogHandler)

	quizRepo := quizzesPG.NewRepository(db)
	quizService := quizzesUC.NewService(quizRepo)
	quizHandler := quizzesHTTP.NewHandler(quizService, quizRepo, courseRepo, catalogRepo)
	quizzesHTTP.RegisterRoutes(e, quizHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}
