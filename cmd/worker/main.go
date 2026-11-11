// cmd/worker es el servicio de Cloud Run mooc-worker (Entrega 4): procesa
// los trabajos multimedia (antivirus, HLS, PDF) que Pub/Sub le entrega por
// push en POST /pubsub/push. Es privado: solo la cuenta de la suscripción
// tiene run.invoker, así que Cloud Run rechaza con 403 cualquier llamada
// sin esa identidad antes de que llegue a este código.
//
// No guarda nada que deba sobrevivir a la instancia: originales y
// derivados van a Cloud Storage y el estado a Postgres. Los archivos
// temporales viven en WORKER_WORKDIR (/tmp) y se borran al terminar cada
// trabajo.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	coursespostgres "github.com/equipo-mooc/plataforma-mooc/internal/courses/postgres"
	mediaplatform "github.com/equipo-mooc/plataforma-mooc/internal/media/platform"
	mediapostgres "github.com/equipo-mooc/plataforma-mooc/internal/media/postgres"
	mediaworker "github.com/equipo-mooc/plataforma-mooc/internal/media/worker"
)

func main() {
	logger := nuevoRegistro(os.Stdout)
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("el worker se detuvo con error", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx := context.Background()

	// ---------- conexiones ----------

	dbConfig, err := pgxpool.ParseConfig(mustEnv("DATABASE_URL"))
	if err != nil {
		return errors.New("DATABASE_URL inválida")
	}
	dbConfig.MaxConns = int32(intEnv("DB_MAX_CONNS", 3))
	dbConfig.MinConns = int32(intEnv("DB_MIN_CONNS", 0))
	dbPool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		return err
	}
	defer dbPool.Close()
	if err := dbPool.Ping(ctx); err != nil {
		return err
	}

	mediaRepo := mediapostgres.NewMediaRepository(dbPool)
	coursesRepo := coursespostgres.NewCourseRepository(dbPool)

	useSSL, _ := strconv.ParseBool(os.Getenv("S3_USE_SSL"))
	storage, err := mediaplatform.NewS3Storage(mediaplatform.S3Config{
		Endpoint: mustEnv("S3_ENDPOINT"),
		// Llaves HMAC de sa-medios, desde Secret Manager.
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    mustEnv("S3_BUCKET"),
		UseSSL:    useSSL,
		Region:    os.Getenv("S3_REGION"),
	})
	if err != nil {
		return err
	}

	// El worker también publica: al terminar el escaneo encadena el
	// siguiente trabajo (HLS o PDF) en el mismo topic.
	queue, err := mediaplatform.NewPubSubQueue(mediaplatform.PubSubConfig{
		ProjectID:    os.Getenv("PUBSUB_PROJECT"),
		TopicID:      os.Getenv("PUBSUB_TOPIC"),
		EmulatorHost: os.Getenv("PUBSUB_EMULATOR_HOST"),
		Timeout:      time.Duration(intEnv("PUBSUB_PUBLISH_TIMEOUT_MS", 5000)) * time.Millisecond,
	})
	if err != nil {
		return err
	}

	workDir := os.Getenv("WORKER_WORKDIR")
	if workDir == "" {
		workDir = "/tmp/media"
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	processor := mediaworker.NewProcessor(mediaRepo, coursesRepo, storage, queue, workDir)

	// Un job en processing cuyo bloqueo no se renueva en LockTTL se da
	// por abandonado (la instancia murió) y la siguiente entrega lo
	// retoma. El latido lo renueva cada LockTTL/3.
	lockTTL := time.Duration(intEnv("WORKER_LOCK_TTL_SECONDS", 60)) * time.Second
	push := &mediaworker.PushHandler{
		Repo:      mediaRepo,
		Runner:    processor,
		Logger:    logger,
		LockTTL:   lockTTL,
		Heartbeat: lockTTL / 3,
	}

	// ---------- servidor HTTP ----------

	estado := &salud{}
	rutas := http.NewServeMux()
	rutas.Handle("POST /pubsub/push", push)
	rutas.Handle("GET /health", estado)

	port := os.Getenv("PORT") // Cloud Run lo define (8080)
	if port == "" {
		port = "8081" // docker-compose: la suscripción del emulador apunta aquí
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           rutas,
		ReadHeaderTimeout: 10 * time.Second,
		// Sin WriteTimeout: un trabajo puede tardar hasta el tiempo
		// máximo de solicitud de Cloud Run (600 s).
	}

	errSrv := make(chan error, 1)
	go func() { errSrv <- srv.ListenAndServe() }()
	logger.Info("worker escuchando", "puerto", port, "bloqueo_ttl_s", lockTTL.Seconds(),
		"revision", os.Getenv("K_REVISION"))

	senal := make(chan os.Signal, 1)
	signal.Notify(senal, syscall.SIGTERM, os.Interrupt)
	select {
	case err := <-errSrv:
		return err
	case s := <-senal:
		// Cloud Run da 10 s tras SIGTERM. Si un trabajo no alcanza a
		// terminar, su bloqueo deja de renovarse y otra instancia lo
		// retoma cuando Pub/Sub lo reentregue.
		logger.Warn("señal de apagado, se esperan los trabajos en curso", "senal", s.String())
		estado.marcarApagando()
		shutdownCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("apagado sin esperar a todos los trabajos", "error", err.Error())
		}
		return nil
	}
}

// mustEnv aborta el arranque si falta una variable requerida, en vez de
// arrancar a medias y fallar de forma confusa en el primer trabajo.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("falta variable de entorno requerida", "variable", key)
		os.Exit(1)
	}
	return v
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
