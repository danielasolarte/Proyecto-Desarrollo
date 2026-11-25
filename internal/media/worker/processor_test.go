package worker

import (
	"errors"
	"testing"

	coursesdomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// repoMedios implementa solo lo que usa HandleJob para validate_upload.
type repoMedios struct {
	mediadomain.MediaRepository
	actualizado *mediadomain.MediaJob
}

func (r *repoMedios) FindAssetByID(id string) (*mediadomain.MediaAsset, error) {
	return &mediadomain.MediaAsset{ID: id, ResourceID: "r1", Kind: mediadomain.MediaKindPDF}, nil
}

func (r *repoMedios) UpdateJob(j *mediadomain.MediaJob) error {
	c := *j
	r.actualizado = &c
	return nil
}

type repoCursos struct {
	coursesdomain.CourseRepository
	estado *coursesdomain.ProcessingStatus
}

func (r *repoCursos) FindResourceByID(id string) (*coursesdomain.Resource, error) {
	return &coursesdomain.Resource{ID: id}, nil
}

func (r *repoCursos) UpdateResource(res *coursesdomain.Resource) error {
	r.estado = res.ProcessingStatus
	return nil
}

// Entrega 5: validate_upload lo ejecuta la Cloud Function. Si por error
// llega al worker, queda en dead_letter al primer intento (sin reintentos).
func TestValidateUploadEnElWorkerQuedaDeadLetter(t *testing.T) {
	medios, cursos := &repoMedios{}, &repoCursos{}
	p := NewProcessor(medios, cursos, nil, nil, t.TempDir())
	job := &mediadomain.MediaJob{ID: "j1", MediaAssetID: "a1", JobType: mediadomain.JobValidateUpload,
		Status: mediadomain.JobProcessing, Attempts: 1, MaxAttempts: 3}

	err := p.HandleJob(job)
	if !errors.Is(err, ErrDeadLettered) {
		t.Fatalf("esperaba ErrDeadLettered, obtuve %v", err)
	}
	if medios.actualizado == nil || medios.actualizado.Status != mediadomain.JobDeadLetter {
		t.Fatalf("el job debía quedar dead_letter: %+v", medios.actualizado)
	}
	if cursos.estado == nil || *cursos.estado != coursesdomain.ProcessingFailed {
		t.Fatalf("el recurso debía quedar failed: %v", cursos.estado)
	}
}
