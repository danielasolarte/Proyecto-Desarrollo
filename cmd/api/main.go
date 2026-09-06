package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	catalogHTTP "github.com/equipo-mooc/plataforma-mooc/internal/catalog/http"
	catalogPG "github.com/equipo-mooc/plataforma-mooc/internal/catalog/postgres"
	coursesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/courses/http"
	coursesPG "github.com/equipo-mooc/plataforma-mooc/internal/courses/postgres"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
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

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// TODO(persona-a): reemplazar este middleware por el real de sesiones
	// cuando internal/auth exista. Ver internal/platform/authctx/authctx.go.
	e.Use(authctx.FakeAuthMiddleware())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	courseRepo := coursesPG.NewCourseRepository(db)
	courseHandler := coursesHTTP.NewHandler(courseRepo)
	coursesHTTP.RegisterRoutes(e, courseHandler)

	catalogRepo := catalogPG.NewCatalogRepository(db)
	catalogHandler := catalogHTTP.NewHandler(catalogRepo)
	catalogHTTP.RegisterRoutes(e, catalogHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}
