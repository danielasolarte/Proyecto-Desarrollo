// Package health expone el estado de una réplica de la API para el
// balanceador y para el grupo de instancias administrado (MIG).
//
//   - /health (vida): responde 200 mientras el proceso esté corriendo. Lo
//     usa la autorreparación del MIG: si deja de responder, la VM se
//     recrea.
//   - /health/ready (listo): responde 200 solo si la réplica puede atender
//     solicitudes (Postgres y Redis responden) y no se está apagando. Lo usa
//     el balanceador para decidir a qué instancias envía tráfico.
//
// Las respuestas nunca incluyen mensajes de error, direcciones ni nombres de
// host de las dependencias: solo "ok" o "error" por chequeo.
package health

import (
	"context"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

// Check verifica una dependencia. Debe respetar el contexto (timeout).
type Check func(ctx context.Context) error

// Checker agrupa los chequeos de preparación y el estado de apagado.
type Checker struct {
	timeout  time.Duration
	draining atomic.Bool

	mu     sync.RWMutex
	checks map[string]Check
}

// New crea un Checker cuyo conjunto de chequeos se ejecuta con el timeout
// indicado (si es 0 se usan 2 segundos).
func New(timeout time.Duration) *Checker {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Checker{timeout: timeout, checks: map[string]Check{}}
}

// Add registra un chequeo de preparación con un nombre estable
// (por ejemplo "postgres" o "redis").
func (h *Checker) Add(name string, check Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// StartDraining marca la réplica como "apagándose": desde ese momento
// /health/ready responde 503 para que el balanceador deje de enviarle
// solicitudes nuevas mientras termina las que tiene en curso.
func (h *Checker) StartDraining() { h.draining.Store(true) }

// Draining indica si la réplica ya está en apagado.
func (h *Checker) Draining() bool { return h.draining.Load() }

// Result es el cuerpo JSON de /health/ready.
type Result struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Ready ejecuta los chequeos en paralelo y devuelve el resultado.
func (h *Checker) Ready(ctx context.Context) (Result, bool) {
	if h.Draining() {
		return Result{Status: "draining"}, false
	}

	h.mu.RLock()
	names := make([]string, 0, len(h.checks))
	for name := range h.checks {
		names = append(names, name)
	}
	sort.Strings(names)
	checks := make([]Check, len(names))
	for i, name := range names {
		checks[i] = h.checks[name]
	}
	h.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	errs := make([]error, len(checks))
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Add(1)
		go func(i int, check Check) {
			defer wg.Done()
			errs[i] = check(ctx)
		}(i, check)
	}
	wg.Wait()

	result := Result{Status: "ready", Checks: make(map[string]string, len(names))}
	ok := true
	for i, name := range names {
		if errs[i] != nil {
			result.Checks[name] = "error"
			ok = false
			continue
		}
		result.Checks[name] = "ok"
	}
	if !ok {
		result.Status = "not_ready"
	}
	return result, ok
}

// Register agrega /health y /health/ready al servidor Echo.
func (h *Checker) Register(e *echo.Echo) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/health/ready", func(c echo.Context) error {
		result, ok := h.Ready(c.Request().Context())
		c.Response().Header().Set("Cache-Control", "no-store")
		if !ok {
			return c.JSON(http.StatusServiceUnavailable, result)
		}
		return c.JSON(http.StatusOK, result)
	})
}

// InstanceID devuelve el identificador técnico de la réplica: la variable
// INSTANCE_ID (el script de arranque la llena con el nombre de la VM) o, si
// no existe, el nombre de host del contenedor.
func InstanceID() string {
	if id := os.Getenv("INSTANCE_ID"); id != "" {
		return id
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return "desconocida"
}

// InstanceHeader agrega X-Instance-Id a todas las respuestas. Sirve como
// evidencia de que el balanceador reparte solicitudes entre réplicas; el
// nombre de la VM no es información sensible.
func InstanceHeader(id string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Instance-Id", id)
			return next(c)
		}
	}
}
