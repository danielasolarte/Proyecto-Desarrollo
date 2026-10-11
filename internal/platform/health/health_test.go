package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func get(t *testing.T, e *echo.Echo, path string) (*httptest.ResponseRecorder, Result) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var r Result
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	return rec, r
}

func TestReadyOKWhenAllChecksPass(t *testing.T) {
	h := New(time.Second)
	h.Add("postgres", func(context.Context) error { return nil })
	h.Add("redis", func(context.Context) error { return nil })
	e := echo.New()
	h.Register(e)

	rec, r := get(t, e, "/health/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rec.Code)
	}
	if r.Status != "ready" || r.Checks["postgres"] != "ok" || r.Checks["redis"] != "ok" {
		t.Fatalf("resultado inesperado: %+v", r)
	}
}

func TestReadyFailsAndHidesErrorDetails(t *testing.T) {
	h := New(time.Second)
	h.Add("postgres", func(context.Context) error {
		return errors.New("dial tcp 10.235.0.3:5432: password authentication failed for user mooc")
	})
	h.Add("redis", func(context.Context) error { return nil })
	e := echo.New()
	h.Register(e)

	rec, r := get(t, e, "/health/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
	if r.Status != "not_ready" || r.Checks["postgres"] != "error" || r.Checks["redis"] != "ok" {
		t.Fatalf("resultado inesperado: %+v", r)
	}
	body := rec.Body.String()
	for _, secreto := range []string{"10.235.0.3", "password", "mooc"} {
		if strings.Contains(body, secreto) {
			t.Fatalf("la respuesta expone %q: %s", secreto, body)
		}
	}
}

func TestReadyRespectsTimeout(t *testing.T) {
	h := New(50 * time.Millisecond)
	h.Add("redis", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	e := echo.New()
	h.Register(e)

	start := time.Now()
	rec, _ := get(t, e, "/health/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("el chequeo no respetó el timeout")
	}
}

func TestDrainingMakesReadyFailButLivenessStaysOK(t *testing.T) {
	h := New(time.Second)
	h.Add("postgres", func(context.Context) error { return nil })
	e := echo.New()
	h.Register(e)

	h.StartDraining()

	rec, r := get(t, e, "/health/ready")
	if rec.Code != http.StatusServiceUnavailable || r.Status != "draining" {
		t.Fatalf("esperaba 503 draining, obtuve %d %+v", rec.Code, r)
	}
	live, _ := get(t, e, "/health")
	if live.Code != http.StatusOK {
		t.Fatalf("/health debe seguir en 200 durante el apagado, obtuve %d", live.Code)
	}
}

func TestInstanceHeader(t *testing.T) {
	t.Setenv("INSTANCE_ID", "mooc-api-abcd")
	e := echo.New()
	e.Use(InstanceHeader(InstanceID()))
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	rec, _ := get(t, e, "/x")
	if got := rec.Header().Get("X-Instance-Id"); got != "mooc-api-abcd" {
		t.Fatalf("X-Instance-Id = %q", got)
	}
}
