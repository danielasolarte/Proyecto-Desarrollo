package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSaludOkYApagando(t *testing.T) {
	s := &salud{}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d, se esperaba 200", rec.Code)
	}

	s.marcarApagando()
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("código %d, se esperaba 503", rec.Code)
	}
}

func TestRegistroFormatoCloudLogging(t *testing.T) {
	var buf bytes.Buffer
	nuevoRegistro(&buf).Warn("job falló", "job_id", "abc")
	var linea map[string]any
	if err := json.Unmarshal(buf.Bytes(), &linea); err != nil {
		t.Fatal(err)
	}
	if linea["severity"] != "WARNING" || linea["message"] != "job falló" || linea["job_id"] != "abc" {
		t.Fatalf("línea %v", linea)
	}
}

func TestIntEnv(t *testing.T) {
	t.Setenv("X_NUM", "7")
	t.Setenv("X_MAL", "siete")
	if intEnv("X_NUM", 1) != 7 || intEnv("X_MAL", 1) != 1 || intEnv("X_NO_EXISTE", 3) != 3 {
		t.Fatal("intEnv no respeta el valor o el respaldo")
	}
}
