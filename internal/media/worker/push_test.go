package worker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

const jobID = "8f0c2c1e-5b8e-4f57-9d3e-1b2a3c4d5e6f"

// claimRepo imita el ClaimJob de Postgres en memoria: toma el job solo si
// está pending/failed o processing con bloqueo vencido.
type claimRepo struct {
	mediadomain.MediaRepository
	mu      sync.Mutex
	jobs    map[string]*mediadomain.MediaJob
	claimEr error
	touches atomic.Int32
}

func (r *claimRepo) ClaimJob(id string, ttl time.Duration) (*mediadomain.MediaJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimEr != nil {
		return nil, r.claimEr
	}
	j, ok := r.jobs[id]
	if !ok {
		return nil, mediadomain.ErrNotFound
	}
	cp := *j
	switch {
	case j.Status == mediadomain.JobDone || j.Status == mediadomain.JobDeadLetter:
		return &cp, mediadomain.ErrJobAlreadyTerminal
	case j.Status == mediadomain.JobProcessing && j.LockedAt != nil && time.Since(*j.LockedAt) < ttl:
		return &cp, mediadomain.ErrJobLocked
	}
	now := time.Now()
	j.Status, j.Attempts, j.LockedAt = mediadomain.JobProcessing, j.Attempts+1, &now
	cp = *j
	return &cp, nil
}

func (r *claimRepo) TouchJobLock(string) error {
	r.touches.Add(1)
	return nil
}

func (r *claimRepo) UpdateJob(j *mediadomain.MediaJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *j
	r.jobs[j.ID] = &cp
	return nil
}

// runner simula el Processor: cuenta ejecuciones y deja el job como lo
// dejaría HandleJob.
type runner struct {
	repo  *claimRepo
	runs  atomic.Int32
	err   error
	delay time.Duration
}

func (f *runner) HandleJob(j *mediadomain.MediaJob) error {
	f.runs.Add(1)
	time.Sleep(f.delay)
	if f.err == nil {
		j.Status, j.LockedAt = mediadomain.JobDone, nil
		return f.repo.UpdateJob(j)
	}
	if j.Attempts >= j.MaxAttempts {
		j.Status = mediadomain.JobDeadLetter
		_ = f.repo.UpdateJob(j)
		return fmt.Errorf("%w: %w", ErrDeadLettered, f.err)
	}
	j.Status, j.LockedAt = mediadomain.JobPending, nil
	_ = f.repo.UpdateJob(j)
	return f.err
}

func newHandler(status mediadomain.JobStatus) (*PushHandler, *claimRepo, *runner) {
	repo := &claimRepo{jobs: map[string]*mediadomain.MediaJob{
		jobID: {ID: jobID, JobType: mediadomain.JobTranscodeHLS, Status: status, MaxAttempts: 3},
	}}
	run := &runner{repo: repo}
	h := &PushHandler{
		Repo: repo, Runner: run, LockTTL: time.Minute,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return h, repo, run
}

func pushBody(t *testing.T, data string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"data":       base64.StdEncoding.EncodeToString([]byte(data)),
			"attributes": map[string]string{"job_type": "transcode_hls", "contract_version": "1"},
			"messageId":  "123",
		},
		"subscription":    "projects/p/subscriptions/mooc-media-jobs-worker",
		"deliveryAttempt": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func send(h http.Handler, body []byte) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pubsub/push", bytes.NewReader(body)))
	return rec.Code
}

func TestPushProcesaYConfirmaDespuesDeGuardar(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobPending)
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusNoContent {
		t.Fatalf("código %d, se esperaba 204", code)
	}
	if run.runs.Load() != 1 || repo.jobs[jobID].Status != mediadomain.JobDone {
		t.Fatalf("ejecuciones %d, estado %s", run.runs.Load(), repo.jobs[jobID].Status)
	}
}

// Prueba de reentrega: el mismo mensaje dos veces procesa una sola vez.
func TestPushReentregaNoDuplica(t *testing.T) {
	h, _, run := newHandler(mediadomain.JobPending)
	body := pushBody(t, `{"job_id":"`+jobID+`"}`)
	for i := 0; i < 2; i++ {
		if code := send(h, body); code != http.StatusNoContent {
			t.Fatalf("entrega %d: código %d", i+1, code)
		}
	}
	if run.runs.Load() != 1 {
		t.Fatalf("se procesó %d veces", run.runs.Load())
	}
}

// Dos entregas simultáneas: una procesa y la otra recibe 429.
func TestPushEntregasSimultaneas(t *testing.T) {
	h, _, run := newHandler(mediadomain.JobPending)
	run.delay = 50 * time.Millisecond
	body := pushBody(t, `{"job_id":"`+jobID+`"}`)
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() { codes <- send(h, body) }()
	}
	got := map[int]int{}
	got[<-codes]++
	got[<-codes]++
	if got[http.StatusNoContent] != 1 || got[http.StatusTooManyRequests] != 1 || run.runs.Load() != 1 {
		t.Fatalf("códigos %v, ejecuciones %d", got, run.runs.Load())
	}
}

func TestPushJobTerminadoNoSeProcesa(t *testing.T) {
	for _, st := range []mediadomain.JobStatus{mediadomain.JobDone, mediadomain.JobDeadLetter} {
		h, _, run := newHandler(st)
		if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusNoContent || run.runs.Load() != 0 {
			t.Fatalf("%s: código %d, ejecuciones %d", st, code, run.runs.Load())
		}
	}
}

func TestPushBloqueoVigenteResponde429(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobProcessing)
	now := time.Now()
	repo.jobs[jobID].LockedAt = &now
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusTooManyRequests || run.runs.Load() != 0 {
		t.Fatalf("código %d, ejecuciones %d", code, run.runs.Load())
	}
}

// Prueba de reemplazo: la instancia murió con el job en processing; cuando
// el bloqueo vence, la siguiente entrega lo retoma y lo termina.
func TestPushRetomaBloqueoVencido(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobProcessing)
	viejo := time.Now().Add(-2 * time.Minute)
	repo.jobs[jobID].LockedAt = &viejo
	repo.jobs[jobID].Attempts = 1
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusNoContent || run.runs.Load() != 1 {
		t.Fatalf("código %d, ejecuciones %d", code, run.runs.Load())
	}
	if j := repo.jobs[jobID]; j.Status != mediadomain.JobDone || j.Attempts != 2 {
		t.Fatalf("estado %s, intentos %d", j.Status, j.Attempts)
	}
}

func TestPushErrorConIntentosResponde500(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobPending)
	run.err = errors.New("ffmpeg falló")
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusInternalServerError {
		t.Fatalf("código %d, se esperaba 500", code)
	}
	if repo.jobs[jobID].Status != mediadomain.JobPending {
		t.Fatalf("estado %s", repo.jobs[jobID].Status)
	}
}

func TestPushSinIntentosQuedaDeadLetterY204(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobPending)
	repo.jobs[jobID].Attempts = 2
	run.err = errors.New("archivo corrupto")
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusNoContent {
		t.Fatalf("código %d, se esperaba 204", code)
	}
	if repo.jobs[jobID].Status != mediadomain.JobDeadLetter {
		t.Fatalf("estado %s", repo.jobs[jobID].Status)
	}
}

func TestPushMensajesInvalidosSeConfirman(t *testing.T) {
	casos := map[string][]byte{
		"no json":        []byte("{"),
		"sin job_id":     pushBody(t, `{}`),
		"job_id no uuid": pushBody(t, `{"job_id":"x'; drop"}`),
		"no existe":      pushBody(t, `{"job_id":"00000000-0000-0000-0000-000000000000"}`),
	}
	for nombre, body := range casos {
		h, _, run := newHandler(mediadomain.JobPending)
		if code := send(h, body); code != http.StatusNoContent || run.runs.Load() != 0 {
			t.Fatalf("%s: código %d, ejecuciones %d", nombre, code, run.runs.Load())
		}
	}
}

// Contrato: un mensaje con campos nuevos (cambio compatible) se acepta.
func TestPushAceptaCamposNuevosDelContrato(t *testing.T) {
	h, _, _ := newHandler(mediadomain.JobPending)
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`","prioridad":"alta"}`)); code != http.StatusNoContent {
		t.Fatalf("código %d", code)
	}
}

func TestPushErrorDeBaseResponde500(t *testing.T) {
	h, repo, _ := newHandler(mediadomain.JobPending)
	repo.claimEr = errors.New("conexión rechazada")
	if code := send(h, pushBody(t, `{"job_id":"`+jobID+`"}`)); code != http.StatusInternalServerError {
		t.Fatalf("código %d", code)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("conexión cortada") }

func TestPushCuerpoIlegibleResponde500(t *testing.T) {
	h, _, _ := newHandler(mediadomain.JobPending)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pubsub/push", errReader{}))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("código %d", rec.Code)
	}
}

func TestPushLatidoRenuevaElBloqueo(t *testing.T) {
	h, repo, run := newHandler(mediadomain.JobPending)
	h.Heartbeat = 10 * time.Millisecond
	run.delay = 55 * time.Millisecond
	send(h, pushBody(t, `{"job_id":"`+jobID+`"}`))
	if repo.touches.Load() < 2 {
		t.Fatalf("latidos %d, se esperaban al menos 2", repo.touches.Load())
	}
	// Al terminar, el latido se detiene.
	n := repo.touches.Load()
	time.Sleep(30 * time.Millisecond)
	if repo.touches.Load() != n {
		t.Fatal("el latido siguió después de terminar el job")
	}
}
