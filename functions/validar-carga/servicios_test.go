package validarcarga

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type tokenFijo string

func (t tokenFijo) Token(context.Context) (string, error) { return string(t), nil }

func TestGCSAlmacen(t *testing.T) {
	var vistos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vistos = append(vistos, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		if r.Header.Get("Authorization") != "Bearer tk" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet:
			io.WriteString(w, "contenido")
		case r.URL.EscapedPath() == "/storage/v1/b/medios/o/ya%2Fborrado":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	g := gcsAlmacen{base: srv.URL, bucket: "medios", cli: srv.Client(), tk: tokenFijo("tk")}

	rc, err := g.Abrir(context.Background(), "resources/r1/original")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "contenido" {
		t.Errorf("contenido = %q", b)
	}
	if err := g.Borrar(context.Background(), "resources/r1/original"); err != nil {
		t.Error(err)
	}
	if err := g.Borrar(context.Background(), "ya/borrado"); err != nil {
		t.Errorf("borrar un objeto que ya no está no es error: %v", err)
	}
	if vistos[0] != "GET /storage/v1/b/medios/o/resources%2Fr1%2Foriginal?alt=media" {
		t.Errorf("la clave debe ir escapada: %s", vistos[0])
	}

	g.tk = tokenFijo("otro")
	if _, err := g.Abrir(context.Background(), "x"); err == nil {
		t.Error("un 401 debe ser error")
	}
}

func TestPubSubColaMismoContrato(t *testing.T) {
	var cuerpo struct {
		Messages []struct {
			Data       string            `json:"data"`
			Attributes map[string]string `json:"attributes"`
		} `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p/topics/mooc-media-jobs:publish" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&cuerpo)
		io.WriteString(w, `{"messageIds":["123"]}`)
	}))
	defer srv.Close()
	c := pubsubCola{url: srv.URL + "/v1/projects/p/topics/mooc-media-jobs:publish", cli: srv.Client(), tk: tokenFijo("tk")}

	if err := c.Publicar(context.Background(), "job-9", tipoEscaneo); err != nil {
		t.Fatal(err)
	}
	if len(cuerpo.Messages) != 1 {
		t.Fatalf("mensajes = %d", len(cuerpo.Messages))
	}
	m := cuerpo.Messages[0]
	data, _ := base64.StdEncoding.DecodeString(m.Data)
	if string(data) != `{"job_id":"job-9"}` {
		t.Errorf("data = %s", data)
	}
	if m.Attributes["job_type"] != "scan_antivirus" || m.Attributes["contract_version"] != "1" {
		t.Errorf("atributos = %v", m.Attributes)
	}
}
