package validarcarga

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
)

type tokenFijo string

func (t tokenFijo) Token(context.Context) (string, error) { return string(t), nil }

func TestBucketGCSLeeYBorraConToken(t *testing.T) {
	var vistos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vistos = append(vistos, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery+" "+r.Header.Get("Authorization"))
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte("contenido"))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	b := &bucketGCS{base: srv.URL, bucket: "medios", client: srv.Client(), tokens: tokenFijo("tok")}
	r, err := b.Abrir(context.Background(), "resources/r1/original.mp4")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	r.Close()
	if string(data) != "contenido" {
		t.Fatalf("leído %q", data)
	}
	if err := b.Borrar(context.Background(), "resources/r1/original.mp4"); err != nil {
		t.Fatal(err)
	}
	if vistos[0] != "GET /storage/v1/b/medios/o/resources%2Fr1%2Foriginal.mp4?alt=media Bearer tok" ||
		!strings.HasPrefix(vistos[1], "DELETE /storage/v1/b/medios/o/resources%2Fr1%2Foriginal.mp4") {
		t.Fatalf("solicitudes %v", vistos)
	}
}

func TestBucketGCSErrores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.Error(w, "prohibido", http.StatusForbidden)
	}))
	defer srv.Close()
	b := &bucketGCS{base: srv.URL, bucket: "m", client: srv.Client(), tokens: tokenFijo("t")}
	if _, err := b.Abrir(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("esperaba 403, obtuve %v", err)
	}
	if err := b.Borrar(context.Background(), "x"); err != nil {
		t.Fatalf("borrar algo que ya no está no es error: %v", err)
	}
}

func TestColaPubSubPublicaElContratoDeLaE4(t *testing.T) {
	var ruta string
	var cuerpo struct {
		Messages []struct {
			Data       string            `json:"data"`
			Attributes map[string]string `json:"attributes"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ruta = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&cuerpo)
		_, _ = w.Write([]byte(`{"messageIds":["1"]}`))
	}))
	defer srv.Close()
	q := nuevaColaPubSub(srv.URL, "p", "mooc-media-jobs", srv.Client(), tokenFijo("t"))
	if err := q.Publicar(context.Background(), "job-9", tipoAntivirus); err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(cuerpo.Messages[0].Data)
	if ruta != "/v1/projects/p/topics/mooc-media-jobs:publish" || string(data) != `{"job_id":"job-9"}` ||
		cuerpo.Messages[0].Attributes["job_type"] != "scan_antivirus" || cuerpo.Messages[0].Attributes["contract_version"] != "1" {
		t.Fatalf("ruta %s, data %s, atributos %v", ruta, data, cuerpo.Messages[0].Attributes)
	}
}

func TestColaPubSubErrorHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sin permiso", http.StatusForbidden)
	}))
	defer srv.Close()
	q := nuevaColaPubSub(srv.URL, "p", "t", srv.Client(), tokenFijo("t"))
	if err := q.Publicar(context.Background(), "j", "x"); err == nil {
		t.Fatal("esperaba error")
	}
}

func TestTokenMetadatosSeGuardaHastaQueVence(t *testing.T) {
	pedidos := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos++
		if r.Header.Get("Metadata-Flavor") != "Google" {
			t.Errorf("falta Metadata-Flavor")
		}
		_, _ = w.Write([]byte(`{"access_token":"abc","expires_in":3600}`))
	}))
	defer srv.Close()
	m := &tokenMetadatos{url: srv.URL, client: srv.Client()}
	for i := 0; i < 3; i++ {
		if tok, err := m.Token(context.Background()); err != nil || tok != "abc" {
			t.Fatalf("token %q %v", tok, err)
		}
	}
	if pedidos != 1 {
		t.Fatalf("pidió el token %d veces", pedidos)
	}
}

func eventoPubSub(t *testing.T, data string) event.Event {
	t.Helper()
	e := event.New()
	e.SetID("1")
	e.SetSource("//pubsub.googleapis.com/projects/p/topics/mooc-media-validate")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	cuerpo := map[string]any{"message": map[string]any{
		"data":       base64.StdEncoding.EncodeToString([]byte(data)),
		"attributes": map[string]string{"job_type": "validate_upload", "contract_version": "1"},
		"messageId":  "m-1",
	}}
	if err := e.SetData("application/json", cuerpo); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLeerJobIDDelCloudEvent(t *testing.T) {
	id, m, err := leerJobID(eventoPubSub(t, `{"job_id":"j-42"}`))
	if err != nil || id != "j-42" || m.Message.Attributes["job_type"] != "validate_upload" || m.Message.MessageID != "m-1" {
		t.Fatalf("id %q, mensaje %+v, error %v", id, m, err)
	}
	if _, _, err := leerJobID(eventoPubSub(t, `{"otro":1}`)); err == nil {
		t.Fatal("un mensaje sin job_id debía dar error")
	}
}

func TestValidarCargaDescartaMensajeInvalidoSinConectarse(t *testing.T) {
	if err := ValidarCarga(context.Background(), eventoPubSub(t, `no es json`)); err != nil {
		t.Fatalf("un mensaje inválido se confirma: %v", err)
	}
}

func TestValidarCargaSinVariablesPideReintento(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "S3_BUCKET", "PUBSUB_PROJECT", "PUBSUB_TOPIC"} {
		t.Setenv(k, "")
	}
	if err := ValidarCarga(context.Background(), eventoPubSub(t, `{"job_id":"j-1"}`)); err == nil {
		t.Fatal("sin configuración debía pedir reintento")
	}
}
