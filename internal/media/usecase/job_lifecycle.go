package usecase

import (
	"errors"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

const defaultMaxAttempts = 3 // "tras tres reintentos fallidos... DLQ" (sección 6)

// EnqueueJob crea un media_job si no existe ya uno con la misma clave de
// idempotencia (así "una entrega duplicada no genera salidas repetidas",
// sección 6), y luego lo publica en la cola real (Redis/asynq) para que un
// worker lo recoja. Si el job ya existía, NO se vuelve a publicar en la
// cola — evita el caso de doble entrega HTTP encolando el mismo trabajo
// dos veces en Redis.
func EnqueueJob(repo domain.MediaRepository, queue domain.JobQueue, mediaAssetID string, jobType domain.JobType, idempotencyKey string) (*domain.MediaJob, error) {
	existing, err := repo.FindJobByIdempotencyKey(idempotencyKey)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	job := &domain.MediaJob{
		MediaAssetID:   mediaAssetID,
		JobType:        jobType,
		Status:         domain.JobPending,
		MaxAttempts:    defaultMaxAttempts,
		IdempotencyKey: idempotencyKey,
	}
	if err := repo.CreateJob(job); err != nil {
		return nil, err
	}
	if err := queue.EnqueueMediaJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

// MarkJobProcessing lo llama el worker justo antes de empezar a trabajar
// el job. Sube el contador de intentos y marca el "lock" de tiempo, que
// sirve para detectar jobs colgados (un worker que murió a mitad de
// camino) y poder reencolarlos.
func MarkJobProcessing(repo domain.MediaRepository, job *domain.MediaJob) error {
	if job.Status == domain.JobDone || job.Status == domain.JobDeadLetter {
		return domain.ErrJobAlreadyTerminal
	}
	now := time.Now()
	job.Status = domain.JobProcessing
	job.Attempts++
	job.LockedAt = &now
	return repo.UpdateJob(job)
}

// MarkJobDone cierra el job exitosamente.
func MarkJobDone(repo domain.MediaRepository, job *domain.MediaJob) error {
	job.Status = domain.JobDone
	job.LockedAt = nil
	job.LastError = nil
	return repo.UpdateJob(job)
}

// MarkJobFailed registra el error. Si ya se agotaron los reintentos, el
// job pasa a dead_letter (y quien orquesta el worker debe emitir la
// alerta correspondiente, ver sección 6). Si todavía quedan intentos,
// vuelve a 'pending' para que asynq lo reintente con backoff.
func MarkJobFailed(repo domain.MediaRepository, job *domain.MediaJob, cause error) error {
	msg := cause.Error()
	job.LastError = &msg
	job.LockedAt = nil

	if job.Attempts >= job.MaxAttempts {
		job.Status = domain.JobDeadLetter
	} else {
		job.Status = domain.JobPending
	}
	return repo.UpdateJob(job)
}
