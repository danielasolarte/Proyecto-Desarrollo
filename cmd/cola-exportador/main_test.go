package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDemandaIncluyeActivos(t *testing.T) {
	// Cola vacía pero con trabajos corriendo: la demanda no puede ser 0,
	// o el autoscaler retiraría workers a mitad de una transcodificación.
	m := muestra{Pendientes: 0, Activos: 2, Reintentos: 5}
	if m.demanda() != 2 {
		t.Fatalf("demanda = %d, esperaba 2", m.demanda())
	}
}

func TestCuerpoSeries(t *testing.T) {
	m := muestra{Pendientes: 3, Activos: 2, Reintentos: 1, Antiguedad: 90 * time.Second}
	cuerpo, err := cuerpoSeries("proyecto-x", "media", m, time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		TimeSeries []struct {
			Metric struct {
				Type   string            `json:"type"`
				Labels map[string]string `json:"labels"`
			} `json:"metric"`
			Resource struct {
				Type   string            `json:"type"`
				Labels map[string]string `json:"labels"`
			} `json:"resource"`
			MetricKind string `json:"metricKind"`
			Points     []struct {
				Interval struct {
					EndTime string `json:"endTime"`
				} `json:"interval"`
				Value map[string]any `json:"value"`
			} `json:"points"`
		} `json:"timeSeries"`
	}
	if err := json.Unmarshal(cuerpo, &doc); err != nil {
		t.Fatal(err)
	}

	esperado := map[string]any{
		prefijoMetrica + "demanda":      "5",
		prefijoMetrica + "pendientes":   "3",
		prefijoMetrica + "activos":      "2",
		prefijoMetrica + "reintentos":   "1",
		prefijoMetrica + "antiguedad_s": 90.0,
	}
	if len(doc.TimeSeries) != len(esperado) {
		t.Fatalf("series = %d, esperaba %d", len(doc.TimeSeries), len(esperado))
	}
	for _, s := range doc.TimeSeries {
		if s.Resource.Type != "global" || s.Resource.Labels["project_id"] != "proyecto-x" {
			t.Errorf("%s: recurso inesperado %+v", s.Metric.Type, s.Resource)
		}
		if s.MetricKind != "GAUGE" || s.Metric.Labels["cola"] != "media" {
			t.Errorf("%s: tipo o etiqueta inesperados", s.Metric.Type)
		}
		if len(s.Points) != 1 || s.Points[0].Interval.EndTime != "2026-10-15T12:00:00Z" {
			t.Fatalf("%s: punto inesperado %+v", s.Metric.Type, s.Points)
		}
		var valor any
		for _, v := range s.Points[0].Value {
			valor = v
		}
		if valor != esperado[s.Metric.Type] {
			t.Errorf("%s = %v, esperaba %v", s.Metric.Type, valor, esperado[s.Metric.Type])
		}
	}
}

func TestPublicarUsaElTokenDeLaVM(t *testing.T) {
	var autorizacion, ruta, recibido string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			if r.Header.Get("Metadata-Flavor") != "Google" {
				http.Error(w, "falta Metadata-Flavor", http.StatusForbidden)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"tok-123","expires_in":3599}`)
		case strings.HasSuffix(r.URL.Path, "/timeSeries"):
			autorizacion, ruta = r.Header.Get("Authorization"), r.URL.Path
			b, _ := io.ReadAll(r.Body)
			recibido = string(b)
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := &publicador{http: srv.Client(), metadatos: srv.URL, monitoring: srv.URL, proyecto: "proyecto-x"}
	if err := p.publicar(context.Background(), []byte(`{"timeSeries":[]}`)); err != nil {
		t.Fatal(err)
	}
	if autorizacion != "Bearer tok-123" || ruta != "/v3/projects/proyecto-x/timeSeries" || recibido != `{"timeSeries":[]}` {
		t.Fatalf("solicitud inesperada: auth=%q ruta=%q cuerpo=%q", autorizacion, ruta, recibido)
	}
}

func TestPublicarReportaElErrorDeMonitoring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			_, _ = io.WriteString(w, `{"access_token":"tok"}`)
			return
		}
		http.Error(w, `{"error":{"message":"Permission monitoring.timeSeries.create denied"}}`, http.StatusForbidden)
	}))
	defer srv.Close()

	p := &publicador{http: srv.Client(), metadatos: srv.URL, monitoring: srv.URL, proyecto: "p"}
	err := p.publicar(context.Background(), []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("esperaba el 403 con su detalle, obtuve %v", err)
	}
}
