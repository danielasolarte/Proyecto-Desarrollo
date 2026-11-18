package usecase

import (
	"testing"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// routingQueue simula una cola que sabe a qué tema mandar ciertos tipos de
// trabajo (como PubSubQueue con PUBSUB_TOPIC_VALIDACION definido).
type routingQueue struct {
	fakeQueue
	routes map[domain.JobType]bool
}

func (q *routingQueue) Routes(t domain.JobType) bool { return q.routes[t] }

func TestFirstJobSinRouterEmpiezaPorAntivirus(t *testing.T) {
	if got := firstJob(&fakeQueue{}); got != domain.JobScanAntivirus {
		t.Fatalf("una cola sin JobRouter debe empezar por scan_antivirus, dio %s", got)
	}
}

func TestFirstJobConFuncionDesplegadaEmpiezaPorValidacion(t *testing.T) {
	q := &routingQueue{routes: map[domain.JobType]bool{domain.JobValidateUpload: true}}
	if got := firstJob(q); got != domain.JobValidateUpload {
		t.Fatalf("con destino para validate_upload debe empezar por ahí, dio %s", got)
	}
}

func TestFirstJobRouterSinDestinoDeValidacion(t *testing.T) {
	q := &routingQueue{routes: map[domain.JobType]bool{}}
	if got := firstJob(q); got != domain.JobScanAntivirus {
		t.Fatalf("sin destino para validate_upload debe empezar por scan_antivirus, dio %s", got)
	}
}
