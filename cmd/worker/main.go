// cmd/worker es el proceso separado que corre los trabajos asíncronos del
// módulo de multimedia (ver diagrama de arquitectura, sección 4 del
// enunciado: "Workers en Go" aparte de la API). Escucha la cola de Redis
// y consume tareas; el único HTTP que expone es GET /health (ver salud.go)
// para la autorreparación del grupo de workers.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	coursespostgres "github.com/equipo-mooc/plataforma-mooc/internal/courses/postgres"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	mediaplatform "github.com/equipo-mooc/plataforma-mooc/internal/media/platform"
	mediapostgres "github.com/equipo-mooc/plataforma-mooc/internal/media/postgres"
	mediaworker "github.com/equipo-mooc/plataforma-mooc/internal/media/worker"
)

// mustEnv aborta el arranque si falta una variable requerida, en vez de
// arrancar "a medias" y fallar de forma confusa más tarde en el primer job.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("falta variable de entorno requerida: %s", key)
	}
	return v
}

func main() {
	ctx := context.Background()

	// ---------- conexiones ----------

	dbConfig, err := pgxpool.ParseConfig(mustEnv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("configuración inválida de postgres: %v", err)
	}

	dbConfig.MaxConns = int32(intEnv("DB_MAX_CONNS", 5))
	dbConfig.MinConns = int32(intEnv("DB_MIN_CONNS", 0))

	dbPool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("conectando a postgres: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}

	mediaRepo := mediapostgres.NewMediaRepository(dbPool)
	// NewCourseRepository ya existe en internal/courses/postgres — se
	// reusa tal cual, el worker necesita leer/escribir resources.
	coursesRepo := coursespostgres.NewCourseRepository(dbPool)

	useSSL, _ := strconv.ParseBool(os.Getenv("S3_USE_SSL"))
	storage, err := mediaplatform.NewS3Storage(mediaplatform.S3Config{
		Endpoint: mustEnv("S3_ENDPOINT"),
		// S3_ACCESS_KEY/S3_SECRET_KEY son las llaves HMAC de la cuenta de
		// servicio de GCS (ver comentario en NewS3Storage); solo existen en
		// el .env real de la VM, nunca en *.env.example ni en el repo.
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
		Bucket:    mustEnv("S3_BUCKET"),
		UseSSL:    useSSL,
		Region:    os.Getenv("S3_REGION"),
	})
	if err != nil {
		log.Fatalf("conectando a minio/s3: %v", err)
	}

	redisAddr := mustEnv("REDIS_ADDR")
	queue := mediaplatform.NewAsynqQueue(redisAddr)
	defer queue.Close()

	workDir := os.Getenv("WORKER_WORKDIR")
	if workDir == "" {
		workDir = "/tmp/media"
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		log.Fatalf("creando directorio de trabajo %s: %v", workDir, err)
	}

	processor := mediaworker.NewProcessor(mediaRepo, coursesRepo, storage, queue, workDir)

	// ---------- servidor de asynq ----------

	concurrency := 5 // valor por defecto: el mismo que había antes
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Fatalf("WORKER_CONCURRENCY inválida %q: debe ser un entero >= 1", v)
		}
		concurrency = n
	}
	log.Printf("worker de multimedia con concurrencia %d", concurrency)

	// Al retirar una VM, el grupo detiene mooc.service y Docker manda
	// SIGTERM. asynq deja de tomar trabajos, espera a los activos hasta
	// ShutdownTimeout y devuelve a la cola los que no alcanzaron a
	// terminar (otro worker los retoma; el estado en Postgres evita
	// duplicados). El timeout debe ser mayor que la duración de un
	// trabajo y menor que el mooc-detener-s de la plantilla.
	shutdownTimeout := time.Duration(intEnv("WORKER_SHUTDOWN_TIMEOUT_SECONDS", 300)) * time.Second
	const intervaloSalud = 15 * time.Second
	estadoSalud := nuevaSalud(intervaloSalud)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			// Cuántos jobs procesa este worker EN PARALELO. El escalamiento
			// horizontal lo hace el grupo de workers (más VM); esto es
			// concurrencia DENTRO de una sola instancia.
			Concurrency: concurrency,
			Queues: map[string]int{
				"media": 1,
			},
			ShutdownTimeout:     shutdownTimeout,
			HealthCheckFunc:     estadoSalud.registrarChequeo,
			HealthCheckInterval: intervaloSalud,
		},
	)

	mux := asynq.NewServeMux()
	for _, jobType := range []mediadomain.JobType{
		mediadomain.JobScanAntivirus,
		mediadomain.JobTranscodeHLS,
		mediadomain.JobConvertPresentationToPDF,
	} {
		mux.HandleFunc(string(jobType), taskHandler(mediaRepo, processor))
	}

	saludAddr := os.Getenv("WORKER_HEALTH_ADDR")
	if saludAddr == "" {
		saludAddr = ":8081"
	}
	rutas := http.NewServeMux()
	rutas.Handle("GET /health", estadoSalud)
	httpSrv := &http.Server{Addr: saludAddr, Handler: rutas, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor de salud en %s: %v", saludAddr, err)
		}
	}()

	if err := srv.Start(mux); err != nil {
		log.Fatalf("el worker no pudo arrancar: %v", err)
	}
	estadoSalud.marcarIniciado()
	log.Printf("worker de multimedia escuchando en la cola 'media' (salud en %s, apagado ordenado hasta %s)", saludAddr, shutdownTimeout)

	senal := make(chan os.Signal, 1)
	signal.Notify(senal, syscall.SIGTERM, os.Interrupt)
	s := <-senal
	log.Printf("señal %v: no se toman trabajos nuevos; se espera hasta %s a los activos", s, shutdownTimeout)
	estadoSalud.marcarApagando()
	srv.Shutdown()
	_ = httpSrv.Close()
	log.Println("worker detenido")
}

// taskHandler adapta la firma que pide asynq (func(ctx, *asynq.Task) error)
// a processor.HandleJob(*domain.MediaJob). El payload que viaja por Redis
// solo trae el ID; el job completo se relee de Postgres, que es la fuente
// de verdad (así si el job cambió de estado entre que se encoló y se
// procesó, el worker ve el estado real, no uno desactualizado).
func taskHandler(repo mediadomain.MediaRepository, processor *mediaworker.Processor) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			JobID string `json:"job_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return err
		}

		job, err := repo.FindJobByID(payload.JobID)
		if err != nil {
			return err
		}

		// Si un procesamiento anterior ya lo cerró (done/dead_letter),
		// no hay nada que hacer: evita reprocesar por una entrega
		// duplicada de Redis (sección 6, "tolerancia a fallos").
		if job.Status == mediadomain.JobDone || job.Status == mediadomain.JobDeadLetter {
			return nil
		}

		return processor.HandleJob(job)
	}
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
