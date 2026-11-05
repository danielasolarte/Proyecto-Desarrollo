package platform

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

type fixedToken string

func (f fixedToken) Token(context.Context) (string, error) { return string(f), nil }

func newTestQueue(t *testing.T, url string) *PubSubQueue {
	t.Helper()
	q, err := NewPubSubQueue(PubSubConfig{ProjectID: "p", TopicID: "t", EmulatorHost: strings.TrimPrefix(url, "http://")})
	if err != nil {
		t.Fatal(err)
	}
	q.backoff = time.Millisecond
	return q
}

func TestPubSubQueuePublishesContract(t *testing.T) {
	var got publishRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/p/topics/t:publish" {
			t.Errorf("solicitud inesperada %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("con el emulador no se envía token")
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"messageIds":["123"]}`))
	}))
	defer srv.Close()

	q := newTestQueue(t, srv.URL)
	job := &domain.MediaJob{ID: "job-1", JobType: domain.JobTranscodeHLS}
	if err := q.EnqueueMediaJob(job); err != nil {
		t.Fatalf("EnqueueMediaJob: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("se esperaba un mensaje, llegaron %d", len(got.Messages))
	}
	m := got.Messages[0]
	if m.Attributes[AttrJobType] != string(domain.JobTranscodeHLS) || m.Attributes[AttrContractVersion] != ContractVersion {
		t.Errorf("atributos inesperados: %v", m.Attributes)
	}
	raw, _ := base64.StdEncoding.DecodeString(m.Data)
	msg, err := DecodeJobMessage(raw)
	if err != nil || msg.JobID != "job-1" {
		t.Errorf("data inesperada %q (%v)", raw, err)
	}
}

func TestPubSubQueueRetriesTransientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "ocupado", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"messageIds":["9"]}`))
	}))
	defer srv.Close()

	q := newTestQueue(t, srv.URL)
	if err := q.EnqueueMediaJob(&domain.MediaJob{ID: "job-2", JobType: domain.JobScanAntivirus}); err != nil {
		t.Fatalf("debía publicar al tercer intento: %v", err)
	}
	if calls != 3 {
		t.Errorf("se esperaban 3 intentos, hubo %d", calls)
	}
}

func TestPubSubQueueDoesNotRetryClientErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"error":"permiso denegado"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	q := newTestQueue(t, srv.URL)
	if err := q.EnqueueMediaJob(&domain.MediaJob{ID: "job-3", JobType: domain.JobScanAntivirus}); err == nil {
		t.Fatal("un 403 debía devolver error")
	}
	if calls != 1 {
		t.Errorf("un 403 no se reintenta; hubo %d intentos", calls)
	}
}

func TestPubSubQueueSendsToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			t.Errorf("token inesperado %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"messageIds":["1"]}`))
	}))
	defer srv.Close()

	q := newTestQueue(t, srv.URL)
	q.tokens = fixedToken("abc")
	if err := q.EnqueueMediaJob(&domain.MediaJob{ID: "job-4", JobType: domain.JobScanAntivirus}); err != nil {
		t.Fatal(err)
	}
}

func TestNewPubSubQueueRequiresTopic(t *testing.T) {
	if _, err := NewPubSubQueue(PubSubConfig{ProjectID: "p"}); err == nil {
		t.Fatal("sin topic debía fallar")
	}
}

func TestMetadataTokenSourceCachesToken(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("Metadata-Flavor") != "Google" {
			t.Errorf("falta Metadata-Flavor")
		}
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	}))
	defer srv.Close()

	m := newMetadataTokenSource(srv.Client())
	m.url = srv.URL
	for i := 0; i < 2; i++ {
		tok, err := m.Token(context.Background())
		if err != nil || tok != "tok" {
			t.Fatalf("token %q, err %v", tok, err)
		}
	}
	if calls != 1 {
		t.Errorf("el token debía quedar en caché; hubo %d llamadas", calls)
	}
}

func TestDecodePushEnvelope(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte(`{"job_id":"job-9"}`))
	body := []byte(`{"message":{"data":"` + data + `","attributes":{"job_type":"transcode_hls"},"messageId":"m1"},"subscription":"projects/p/subscriptions/s","deliveryAttempt":2}`)
	msg, env, err := DecodePushEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	if msg.JobID != "job-9" || env.Message.MessageID != "m1" || env.DeliveryAttempt != 2 || env.Message.Attributes[AttrJobType] != "transcode_hls" {
		t.Errorf("sobre mal decodificado: %+v %+v", msg, env)
	}
}

func TestDecodePushEnvelopeRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"json roto":    `{`,
		"no base64":    `{"message":{"data":"%%%"}}`,
		"sin job_id":   `{"message":{"data":"` + base64.StdEncoding.EncodeToString([]byte(`{}`)) + `"}}`,
		"job_id vacío": `{"message":{"data":"` + base64.StdEncoding.EncodeToString([]byte(`{"job_id":"  "}`)) + `"}}`,
	}
	for name, body := range cases {
		if _, _, err := DecodePushEnvelope([]byte(body)); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: se esperaba ErrInvalidMessage, llegó %v", name, err)
		}
	}
}

func TestEmulatorBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"pubsub:8085":        "http://pubsub:8085",
		"http://pubsub:8085": "http://pubsub:8085",
		"localhost:8085/":    "http://localhost:8085",
	} {
		if got := emulatorBaseURL(in); got != want {
			t.Errorf("emulatorBaseURL(%q) = %q, se esperaba %q", in, got, want)
		}
	}
}

func TestNewPubSubQueueUsesGoogleWithoutEmulator(t *testing.T) {
	q, err := NewPubSubQueue(PubSubConfig{ProjectID: "p", TopicID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if q.publishURL != "https://pubsub.googleapis.com/v1/projects/p/topics/t:publish" || q.tokens == nil || q.timeout != 5*time.Second {
		t.Errorf("configuración inesperada: %s, token %v, timeout %s", q.publishURL, q.tokens != nil, q.timeout)
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPubSubQueueRejectsResponseWithoutIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if err := newTestQueue(t, srv.URL).EnqueueMediaJob(&domain.MediaJob{ID: "j", JobType: domain.JobScanAntivirus}); err == nil {
		t.Fatal("una respuesta sin messageIds debía ser error")
	}
}

type failingToken struct{}

func (failingToken) Token(context.Context) (string, error) { return "", errors.New("sin metadatos") }

func TestPubSubQueueRetriesWhenTokenFails(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()
	q := newTestQueue(t, srv.URL)
	q.tokens = failingToken{}
	if err := q.EnqueueMediaJob(&domain.MediaJob{ID: "j", JobType: domain.JobScanAntivirus}); err == nil {
		t.Fatal("sin token debía fallar")
	}
	if calls != 0 {
		t.Errorf("sin token no se debía llamar a Pub/Sub; hubo %d llamadas", calls)
	}
}

func TestMetadataTokenSourceErrors(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"500":       func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", http.StatusInternalServerError) },
		"json roto": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{`)) },
		"sin token": func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"expires_in":10}`)) },
	}
	for name, h := range cases {
		srv := httptest.NewServer(h)
		m := newMetadataTokenSource(srv.Client())
		m.url = srv.URL
		if _, err := m.Token(context.Background()); err == nil {
			t.Errorf("%s: se esperaba error", name)
		}
		srv.Close()
	}
	m := newMetadataTokenSource(nil)
	m.url = "http://127.0.0.1:1/token"
	if _, err := m.Token(context.Background()); err == nil {
		t.Error("sin servidor de metadatos se esperaba error")
	}
}
