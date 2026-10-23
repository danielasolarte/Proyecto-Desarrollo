package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSaludEstados(t *testing.T) {
	reloj := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s := nuevaSalud(15 * time.Second)
	s.ahora = func() time.Time { return reloj }

	pasos := []struct {
		nombre string
		accion func()
		estado string
		codigo int
	}{
		{"antes de arrancar", func() {}, "iniciando", http.StatusServiceUnavailable},
		{"arrancado sin chequeo", s.marcarIniciado, "iniciando", http.StatusServiceUnavailable},
		{"redis responde", func() { s.registrarChequeo(nil) }, "ok", http.StatusOK},
		{"redis falla", func() { s.registrarChequeo(errors.New("dial tcp: timeout")) }, "redis_no_disponible", http.StatusServiceUnavailable},
		{"redis vuelve", func() { s.registrarChequeo(nil) }, "ok", http.StatusOK},
		{"chequeo viejo", func() { reloj = reloj.Add(46 * time.Second) }, "sin_chequeo_reciente", http.StatusServiceUnavailable},
		{"chequeo nuevo", func() { s.registrarChequeo(nil) }, "ok", http.StatusOK},
		{"apagando", s.marcarApagando, "apagando", http.StatusServiceUnavailable},
	}

	for _, p := range pasos {
		p.accion()
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != p.codigo || !strings.Contains(rec.Body.String(), `"status":"`+p.estado+`"`) {
			t.Fatalf("%s: esperaba %d %q, obtuve %d %s", p.nombre, p.codigo, p.estado, rec.Code, rec.Body.String())
		}
	}
}

func TestSaludNoExponeElError(t *testing.T) {
	s := nuevaSalud(time.Second)
	s.marcarIniciado()
	s.registrarChequeo(errors.New("dial tcp 10.20.0.7:6379: connection refused"))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if strings.Contains(rec.Body.String(), "10.20.0.7") {
		t.Fatalf("la respuesta expone detalles internos: %s", rec.Body.String())
	}
}
