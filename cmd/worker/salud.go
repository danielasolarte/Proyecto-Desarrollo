package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// salud responde GET /health: 200 mientras el worker atiende y 503 cuando
// recibió la señal de apagado. En Cloud Run el arranque lo controla la
// prueba de inicio (el puerto solo abre cuando clamd ya respondió, ver
// deploy/worker-entrypoint.sh); esto sirve en docker-compose y para
// diagnóstico.
type salud struct {
	apagando atomic.Bool
}

func (s *salud) marcarApagando() { s.apagando.Store(true) }

func (s *salud) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	estado := "ok"
	w.Header().Set("Content-Type", "application/json")
	if s.apagando.Load() {
		estado = "apagando"
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": estado})
}
