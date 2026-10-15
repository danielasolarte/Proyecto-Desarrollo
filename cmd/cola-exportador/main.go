// cmd/cola-exportador publica cada INTERVALO_S segundos el estado de la cola
// "media" en Cloud Monitoring. Es la señal del autoscaling del grupo de
// workers: la CPU no sirve porque ffmpeg la satura con un solo trabajo, así
// que no distingue entre 1 y 20 trabajos en espera.
//
// Corre UNA sola copia, en la VM de servicios (fuera de los grupos): si cada
// worker publicara la métrica, habría varias series para el mismo valor.
//
// Métricas (tipo GAUGE, recurso "global"):
//
//	custom.googleapis.com/mooc/cola/demanda       pendientes + activos (señal del autoscaling)
//	custom.googleapis.com/mooc/cola/pendientes    esperando un worker
//	custom.googleapis.com/mooc/cola/activos       en procesamiento
//	custom.googleapis.com/mooc/cola/reintentos    esperando su siguiente intento
//	custom.googleapis.com/mooc/cola/antiguedad_s  edad del trabajo pendiente más viejo, en segundos
//
// La demanda incluye los activos a propósito: una cola sin pendientes puede
// tener trabajos en ejecución, y retirar esos workers los cortaría.
//
// Cada muestra también se escribe en el log (una línea por intervalo), que
// sirve de evidencia para la línea de tiempo del Escenario 3B.
//
// Sin acceso al servidor de metadatos de GCP (por ejemplo, en local) solo
// escribe el log. Se autentica con la cuenta de servicio de la VM, que
// necesita roles/monitoring.metricWriter.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
)

const prefijoMetrica = "custom.googleapis.com/mooc/cola/"

type muestra struct {
	Pendientes int
	Activos    int
	Reintentos int
	Antiguedad time.Duration
}

func (m muestra) demanda() int { return m.Pendientes + m.Activos }

func leerCola(insp *asynq.Inspector, cola string) (muestra, error) {
	info, err := insp.GetQueueInfo(cola)
	if errors.Is(err, asynq.ErrQueueNotFound) {
		// La cola aún no existe en Redis porque nunca se encoló nada.
		return muestra{}, nil
	}
	if err != nil {
		return muestra{}, err
	}
	return muestra{
		Pendientes: info.Pending,
		Activos:    info.Active,
		Reintentos: info.Retry,
		Antiguedad: info.Latency,
	}, nil
}

// cuerpoSeries arma el JSON de projects.timeSeries.create con una serie por
// métrica, todas con el mismo instante.
func cuerpoSeries(proyecto, cola string, m muestra, t time.Time) ([]byte, error) {
	instante := t.UTC().Format(time.RFC3339Nano)
	entero := func(nombre string, v int) map[string]any {
		return serie(proyecto, cola, nombre, instante, map[string]any{"int64Value": strconv.Itoa(v)})
	}
	series := []map[string]any{
		entero("demanda", m.demanda()),
		entero("pendientes", m.Pendientes),
		entero("activos", m.Activos),
		entero("reintentos", m.Reintentos),
		serie(proyecto, cola, "antiguedad_s", instante, map[string]any{"doubleValue": m.Antiguedad.Seconds()}),
	}
	return json.Marshal(map[string]any{"timeSeries": series})
}

func serie(proyecto, cola, nombre, instante string, valor map[string]any) map[string]any {
	return map[string]any{
		"metric":     map[string]any{"type": prefijoMetrica + nombre, "labels": map[string]string{"cola": cola}},
		"resource":   map[string]any{"type": "global", "labels": map[string]string{"project_id": proyecto}},
		"metricKind": "GAUGE",
		"points":     []map[string]any{{"interval": map[string]string{"endTime": instante}, "value": valor}},
	}
}

// publicador escribe en Cloud Monitoring con el token de la cuenta de
// servicio que entrega el servidor de metadatos de la VM.
type publicador struct {
	http       *http.Client
	metadatos  string
	monitoring string
	proyecto   string
}

func (p *publicador) leerMetadato(ctx context.Context, ruta string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.metadatos+ruta, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadatos %s: HTTP %d", ruta, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (p *publicador) token(ctx context.Context) (string, error) {
	cuerpo, err := p.leerMetadato(ctx, "/instance/service-accounts/default/token")
	if err != nil {
		return "", err
	}
	var t struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(cuerpo, &t); err != nil || t.AccessToken == "" {
		return "", fmt.Errorf("token de metadatos inválido")
	}
	return t.AccessToken, nil
}

func (p *publicador) publicar(ctx context.Context, cuerpo []byte) error {
	token, err := p.token(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v3/projects/%s/timeSeries", p.monitoring, p.proyecto)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(cuerpo))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detalle, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("cloud monitoring: HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(detalle))
	}
	return nil
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("falta variable de entorno requerida: REDIS_ADDR")
	}
	cola := os.Getenv("COLA")
	if cola == "" {
		cola = "media"
	}
	intervalo := 30 * time.Second
	if v, err := strconv.Atoi(os.Getenv("INTERVALO_S")); err == nil && v >= 10 {
		intervalo = time.Duration(v) * time.Second
	}

	p := &publicador{
		http:       &http.Client{Timeout: 10 * time.Second},
		metadatos:  envODefecto("MOOC_MD", "http://metadata.google.internal/computeMetadata/v1"),
		monitoring: envODefecto("MOOC_MONITORING", "https://monitoring.googleapis.com"),
	}
	ctx, cancelar := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancelar()

	if id, err := p.leerMetadato(ctx, "/project/project-id"); err == nil {
		p.proyecto = string(id)
		log.Printf("publicando la cola %q en Cloud Monitoring (proyecto %s) cada %s", cola, p.proyecto, intervalo)
	} else {
		log.Printf("sin servidor de metadatos (%v): solo se escribe el log", err)
	}

	insp := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
	defer insp.Close()

	tic := time.NewTicker(intervalo)
	defer tic.Stop()
	for {
		ahora := time.Now()
		if m, err := leerCola(insp, cola); err != nil {
			log.Printf("leyendo la cola %q: %v", cola, err)
		} else {
			log.Printf("cola=%s demanda=%d pendientes=%d activos=%d reintentos=%d antiguedad_s=%.0f",
				cola, m.demanda(), m.Pendientes, m.Activos, m.Reintentos, m.Antiguedad.Seconds())
			if p.proyecto != "" {
				cuerpo, err := cuerpoSeries(p.proyecto, cola, m, ahora)
				if err == nil {
					err = p.publicar(ctx, cuerpo)
				}
				if err != nil {
					log.Printf("publicando en Cloud Monitoring: %v", err)
				}
			}
		}
		select {
		case <-ctx.Done():
			log.Println("exportador detenido")
			return
		case <-tic.C:
		}
	}
}

func envODefecto(clave, defecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return defecto
}
