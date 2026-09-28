package platform

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// AsynqQueue implementa domain.JobQueue publicando en Redis vía asynq. El
// "tipo de tarea" que asynq usa para enrutar al handler correcto es
// justamente el JobType del dominio (ej. "scan_antivirus"), así que no
// hace falta un mapeo aparte: ver cmd/worker/main.go donde se registra un
// handler por cada JobType usando el mismo string.
type AsynqQueue struct {
	client *asynq.Client
}

func NewAsynqQueue(redisAddr string) *AsynqQueue {
	return &AsynqQueue{client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})}
}

func (q *AsynqQueue) Close() error {
	return q.client.Close()
}

// jobPayload es deliberadamente mínimo: solo el ID. El worker vuelve a
// leer el media_job completo desde Postgres antes de procesarlo (fuente
// de verdad), en vez de confiar en lo que viajó por Redis, que podría
// quedar desactualizado si el job se reintenta.
type jobPayload struct {
	JobID string `json:"job_id"`
}

func (q *AsynqQueue) EnqueueMediaJob(job *domain.MediaJob) error {
	payload, err := json.Marshal(jobPayload{JobID: job.ID})
	if err != nil {
		return err
	}

	task := asynq.NewTask(string(job.JobType), payload)
	_, err = q.client.Enqueue(task,
		asynq.Queue("media"),
		// job.MaxAttempts ya cuenta el primer intento; MaxRetry de asynq
		// cuenta SOLO los reintentos adicionales.
		asynq.MaxRetry(job.MaxAttempts-1),
		// Un video largo puede tardar varios minutos en transcodificar;
		// sin este timeout asynq podría considerar el task "perdido" y
		// reencolarlo mientras el worker todavía lo está procesando.
		asynq.Timeout(30*time.Minute),
	)
	return err
}
