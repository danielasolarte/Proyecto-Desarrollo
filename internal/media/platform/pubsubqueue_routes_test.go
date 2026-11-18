package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

func TestPubSubQueueRoutesValidationToItsTopic(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"messageIds":["1"]}`))
	}))
	defer srv.Close()

	q, err := NewPubSubQueue(PubSubConfig{
		ProjectID:       "p",
		TopicID:         "trabajos",
		EmulatorHost:    strings.TrimPrefix(srv.URL, "http://"),
		TopicsByJobType: map[domain.JobType]string{domain.JobValidateUpload: "validacion"},
	})
	if err != nil {
		t.Fatal(err)
	}
	q.backoff = time.Millisecond

	if !q.Routes(domain.JobValidateUpload) || q.Routes(domain.JobScanAntivirus) {
		t.Fatalf("Routes debe ser true solo para validate_upload")
	}
	for _, jt := range []domain.JobType{domain.JobValidateUpload, domain.JobScanAntivirus} {
		if err := q.EnqueueMediaJob(&domain.MediaJob{ID: "j-" + string(jt), JobType: jt}); err != nil {
			t.Fatalf("EnqueueMediaJob(%s): %v", jt, err)
		}
	}
	want := []string{"/v1/projects/p/topics/validacion:publish", "/v1/projects/p/topics/trabajos:publish"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("rutas publicadas %v, se esperaban %v", paths, want)
	}
}

func TestPubSubQueueIgnoresEmptyRoute(t *testing.T) {
	q, err := NewPubSubQueue(PubSubConfig{
		ProjectID:       "p",
		TopicID:         "trabajos",
		EmulatorHost:    "localhost:1",
		TopicsByJobType: map[domain.JobType]string{domain.JobValidateUpload: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q.Routes(domain.JobValidateUpload) {
		t.Fatal("un tema vacío no debe activar la ruta")
	}
	if q.urlFor(domain.JobValidateUpload) != q.publishURL {
		t.Fatal("sin ruta, validate_upload debe ir al tema por defecto")
	}
}
