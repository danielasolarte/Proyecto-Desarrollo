package worker

import (
	"errors"
	"testing"

	coursesdomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// Dobles mínimos: embeben la interfaz y solo implementan lo que HandleJob
// usa en este camino.
type assetRepo struct {
	mediadomain.MediaRepository
	asset   *mediadomain.MediaAsset
	updates []mediadomain.MediaJob
}

func (r *assetRepo) FindAssetByID(string) (*mediadomain.MediaAsset, error) { return r.asset, nil }

func (r *assetRepo) UpdateJob(j *mediadomain.MediaJob) error {
	r.updates = append(r.updates, *j)
	return nil
}

type resourceRepo struct {
	coursesdomain.CourseRepository
	res *coursesdomain.Resource
}

func (r *resourceRepo) FindResourceByID(string) (*coursesdomain.Resource, error) { return r.res, nil }
func (r *resourceRepo) UpdateResource(res *coursesdomain.Resource) error         { r.res = res; return nil }

// Entrega 5: validate_upload lo hace la Cloud Function. Si por error llega
// al worker, no se reintenta: queda en dead_letter al primer intento.
func TestWorkerRechazaValidateUpload(t *testing.T) {
	media := &assetRepo{asset: &mediadomain.MediaAsset{ID: "a1", ResourceID: "r1", Kind: mediadomain.MediaKindPDF}}
	courses := &resourceRepo{res: &coursesdomain.Resource{ID: "r1"}}
	p := NewProcessor(media, courses, nil, nil, t.TempDir())

	job := &mediadomain.MediaJob{ID: "j1", MediaAssetID: "a1", JobType: mediadomain.JobValidateUpload,
		Status: mediadomain.JobProcessing, Attempts: 1, MaxAttempts: 3}
	err := p.HandleJob(job)

	if !errors.Is(err, ErrDeadLettered) {
		t.Fatalf("HandleJob = %v, quiero ErrDeadLettered (push responde 204)", err)
	}
	if job.Status != mediadomain.JobDeadLetter || job.LastError == nil {
		t.Errorf("job = %s, last_error = %v; quiero dead_letter con mensaje", job.Status, job.LastError)
	}
	if s := courses.res.ProcessingStatus; s == nil || *s != coursesdomain.ProcessingFailed {
		t.Errorf("recurso = %v, quiero failed", s)
	}
}
