package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// salud responde GET /health para la autorreparación del grupo de workers.
// El worker está sano cuando el servidor de asynq arrancó, el último ping a
// Redis (HealthCheckFunc de asynq) salió bien y es reciente, y no se está
// apagando. No revisa Postgres: una caída breve de Cloud SQL haría que el
// grupo recreara todos los workers a la vez sin ganar nada.
type salud struct {
	mu       sync.Mutex
	iniciado bool
	apagando bool
	ultimo   time.Time
	errRedis error
	vigencia time.Duration // un chequeo más viejo que esto ya no cuenta
	ahora    func() time.Time
}

func nuevaSalud(intervalo time.Duration) *salud {
	return &salud{vigencia: 3 * intervalo, ahora: time.Now}
}

// registrarChequeo es el HealthCheckFunc de asynq: se llama en cada ping a
// Redis, con err == nil si respondió.
func (s *salud) registrarChequeo(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ultimo = s.ahora()
	s.errRedis = err
}

func (s *salud) marcarIniciado() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.iniciado = true
}

func (s *salud) marcarApagando() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apagando = true
}

func (s *salud) estado() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.apagando:
		return "apagando"
	case !s.iniciado || s.ultimo.IsZero():
		return "iniciando"
	case s.errRedis != nil:
		return "redis_no_disponible"
	case s.ahora().Sub(s.ultimo) > s.vigencia:
		return "sin_chequeo_reciente"
	default:
		return "ok"
	}
}

func (s *salud) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	estado := s.estado()
	w.Header().Set("Content-Type", "application/json")
	if estado != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": estado})
}
