package usecase

import (
	"errors"
	"testing"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// jobRepo es un MediaRepository en memoria con solo lo que usa EnqueueJob.
type jobRepo struct {
	domain.MediaRepository
	byKey   map[string]*domain.MediaJob
	created int
}

func newJobRepo() *jobRepo { return &jobRepo{byKey: map[string]*domain.MediaJob{}} }

func (r *jobRepo) FindJobByIdempotencyKey(key string) (*domain.MediaJob, error) {
	if j, ok := r.byKey[key]; ok {
		return j, nil
	}
	return nil, domain.ErrNotFound
}

func (r *jobRepo) CreateJob(j *domain.MediaJob) error {
	r.created++
	j.ID = "job-" + j.IdempotencyKey
	r.byKey[j.IdempotencyKey] = j
	return nil
}

type fakeQueue struct {
	published []string
	fail      bool
}

func (q *fakeQueue) EnqueueMediaJob(j *domain.MediaJob) error {
	if q.fail {
		return errors.New("pubsub caído")
	}
	q.published = append(q.published, j.ID)
	return nil
}

func TestEnqueueJobCreatesAndPublishes(t *testing.T) {
	repo, queue := newJobRepo(), &fakeQueue{}
	job, err := EnqueueJob(repo, queue, "asset-1", domain.JobScanAntivirus, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if repo.created != 1 || len(queue.published) != 1 || queue.published[0] != job.ID {
		t.Fatalf("creados %d, publicados %v", repo.created, queue.published)
	}
}

// Si publicar falló la primera vez, el job queda pending en Postgres; al
// repetir la operación se publica de nuevo sin crear otro job.
func TestEnqueueJobRepublishesPendingJob(t *testing.T) {
	repo, queue := newJobRepo(), &fakeQueue{fail: true}
	if _, err := EnqueueJob(repo, queue, "asset-1", domain.JobScanAntivirus, "k1"); err == nil {
		t.Fatal("debía fallar mientras Pub/Sub está caído")
	}
	queue.fail = false
	job, err := EnqueueJob(repo, queue, "asset-1", domain.JobScanAntivirus, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if repo.created != 1 {
		t.Errorf("no debía crear un segundo job; creados %d", repo.created)
	}
	if len(queue.published) != 1 || queue.published[0] != job.ID {
		t.Errorf("debía publicar el job pendiente; publicados %v", queue.published)
	}
}

func TestEnqueueJobDoesNotRepublishStartedJobs(t *testing.T) {
	for _, status := range []domain.JobStatus{domain.JobProcessing, domain.JobDone, domain.JobDeadLetter} {
		repo, queue := newJobRepo(), &fakeQueue{}
		repo.byKey["k1"] = &domain.MediaJob{ID: "job-k1", Status: status, IdempotencyKey: "k1"}
		if _, err := EnqueueJob(repo, queue, "asset-1", domain.JobScanAntivirus, "k1"); err != nil {
			t.Fatal(err)
		}
		if len(queue.published) != 0 {
			t.Errorf("estado %s: no debía publicar de nuevo", status)
		}
	}
}
