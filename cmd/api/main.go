package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"

	adminHTTP "github.com/equipo-mooc/plataforma-mooc/internal/admin/http"
	authHTTP "github.com/equipo-mooc/plataforma-mooc/internal/auth/http"
	authPG "github.com/equipo-mooc/plataforma-mooc/internal/auth/postgres"
	badgesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/badges/http"
	badgesPG "github.com/equipo-mooc/plataforma-mooc/internal/badges/postgres"
	badgesUC "github.com/equipo-mooc/plataforma-mooc/internal/badges/usecase"
	catalogHTTP "github.com/equipo-mooc/plataforma-mooc/internal/catalog/http"
	catalogPG "github.com/equipo-mooc/plataforma-mooc/internal/catalog/postgres"
	coursesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/courses/http"
	coursesPG "github.com/equipo-mooc/plataforma-mooc/internal/courses/postgres"
	mediaHTTP "github.com/equipo-mooc/plataforma-mooc/internal/media/http"
	mediaPlatform "github.com/equipo-mooc/plataforma-mooc/internal/media/platform"
	mediaPG "github.com/equipo-mooc/plataforma-mooc/internal/media/postgres"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/mailer"
	progressHTTP "github.com/equipo-mooc/plataforma-mooc/internal/progress/http"
	progressPG "github.com/equipo-mooc/plataforma-mooc/internal/progress/postgres"
	progressUC "github.com/equipo-mooc/plataforma-mooc/internal/progress/usecase"
	quizzesHTTP "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/http"
	quizzesPG "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/postgres"
	quizzesUC "github.com/equipo-mooc/plataforma-mooc/internal/quizzes/usecase"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable"
	}

	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("configuración inválida de postgres: %v", err)
	}

	dbConfig.MaxConns = int32(intEnv("DB_MAX_CONNS", 5))
	dbConfig.MinConns = int32(intEnv("DB_MIN_CONNS", 0))

	db, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}

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

	// Progress repository
	progressRepo := progressPG.NewRepository(db)

	// Media repository
	mediaRepo := mediaPG.NewMediaRepository(db)

	// Badges
	badgeRepo := badgesPG.NewRepository(db)
	badgeService := badgesUC.NewService(badgeRepo, progressRepo)

	// Progress service
	progressService := progressUC.NewService(progressRepo, courseRepo, badgeService, mediaRepo)

	// Progress HTTP
	progressHandler := progressHTTP.NewHandler(progressService, courseRepo, catalogRepo)
	progressHTTP.RegisterRoutes(e, progressHandler)

	// Badges HTTP
	badgeHandler := badgesHTTP.NewHandler(badgeService, courseRepo)
	badgesHTTP.RegisterRoutes(e, badgeHandler)

	// Quizzes
	quizRepo := quizzesPG.NewRepository(db)
	quizService := quizzesUC.NewService(quizRepo)
	quizHandler := quizzesHTTP.NewHandler(quizService, quizRepo, courseRepo, catalogRepo, progressService)
	quizzesHTTP.RegisterRoutes(e, quizHandler)

	// Media: storage (MinIO/S3) y cola de trabajos (Redis/asynq) son
	// infraestructura propia del módulo, aparte de userRepo/sessionStore
	// que ya usan los demás.
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		s3Endpoint = "localhost:9000"
	}
	// S3_ACCESS_KEY/S3_SECRET_KEY son las llaves HMAC de la cuenta de
	// servicio de Cloud Storage (ver comentario en NewS3Storage). No se
	// rellena un valor por defecto aquí a propósito: un fallback local
	// (las credenciales de MinIO) terminaría intentando autenticarse
	// contra Cloud Storage real con credenciales inválidas.
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "mooc-media"
	}
	s3UseSSL, _ := strconv.ParseBool(os.Getenv("S3_USE_SSL"))
	s3Region := os.Getenv("S3_REGION")

	// S3_PUBLIC_ENDPOINT es el host:puerto que queda firmado dentro de las
	// URLs prefirmadas que recibe un cliente externo (navegador, Postman,
	// reproductor HLS) -- distinto de S3_ENDPOINT cuando la API corre
	// dentro de docker-compose (host interno "minio:9000") pero quien sube
	// o reproduce el archivo está fuera de esa red (ver S3Config en
	// internal/media/platform/s3storage.go). Si no se define, se usa el
	// mismo S3_ENDPOINT de siempre.
	s3PublicEndpoint := os.Getenv("S3_PUBLIC_ENDPOINT")
	s3PublicUseSSL := s3UseSSL
	if v := os.Getenv("S3_PUBLIC_USE_SSL"); v != "" {
		s3PublicUseSSL, _ = strconv.ParseBool(v)
	}

	mediaStorage, err := mediaPlatform.NewS3Storage(mediaPlatform.S3Config{
		Endpoint:       s3Endpoint,
		AccessKey:      s3AccessKey,
		SecretKey:      s3SecretKey,
		Bucket:         s3Bucket,
		UseSSL:         s3UseSSL,
		PublicEndpoint: s3PublicEndpoint,
		PublicUseSSL:   s3PublicUseSSL,
		Region:         s3Region,
	})
	if err != nil {
		log.Fatalf("no se pudo conectar a minio/s3: %v", err)
	}

	mediaQueue := mediaPlatform.NewAsynqQueue(redisAddr)
	defer mediaQueue.Close()

	mediaHandler := mediaHTTP.NewHandler(mediaRepo, courseRepo, catalogRepo, mediaStorage, mediaQueue)
	mediaHTTP.RegisterRoutes(e, mediaHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}
func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
