package main

import (
	"log"
	"os"

	"github.com/hibiken/asynq"
)

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{Concurrency: 5},
	)

	mux := asynq.NewServeMux()
	// El módulo de cursos/catálogo no encola trabajos asíncronos propios
	// todavía. Los handlers de transcodificación HLS y antimalware los
	// registra Persona C (internal/media) aquí mismo cuando su módulo
	// esté listo.

	if err := srv.Run(mux); err != nil {
		log.Fatalf("no se pudo iniciar el worker: %v", err)
	}
}
