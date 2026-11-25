// Package validarcarga es la Cloud Function mooc-validar-carga (Entrega 5).
//
// Recibe cada validate_upload que publica el backend en
// mooc-media-validate (Eventarc), valida el original (SHA-256 real, MIME
// real y firma ZIP de las presentaciones) y, si pasa, encola scan_antivirus
// en mooc-media-jobs para el worker. Contrato, idempotencia y reintentos:
// docs/entrega5/contrato-validacion.md.
package validarcarga

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	// El main que genera Cloud Build importa funcframework. Se importa aquí
	// también para que go.mod y go.sum lleven todas sus dependencias.
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/jackc/pgx/v5/pgxpool"
)

func init() {
	functions.CloudEvent("ValidarCarga", ValidarCarga)
}

// mensajePubSub es el cuerpo del CloudEvent
// google.cloud.pubsub.topic.v1.messagePublished.
type mensajePubSub struct {
	Message struct {
		Data       []byte            `json:"data"`
		Attributes map[string]string `json:"attributes"`
		MessageID  string            `json:"messageId"`
	} `json:"message"`
}

// leerJobID saca el job_id del evento. Un mensaje que nunca se podrá
// procesar devuelve "" (y se confirma).
func leerJobID(e event.Event) (string, mensajePubSub, error) {
	var m mensajePubSub
	if err := e.DataAs(&m); err != nil {
		return "", m, fmt.Errorf("evento de Pub/Sub ilegible: %w", err)
	}
	var cuerpo struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(m.Message.Data, &cuerpo); err != nil || cuerpo.JobID == "" {
		return "", m, fmt.Errorf("mensaje sin job_id")
	}
	return cuerpo.JobID, m, nil
}

var (
	mu     sync.Mutex
	global *validador
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: severidad}))
)

// dependencias arma el validador la primera vez y lo reutiliza en las
// siguientes invocaciones de la misma instancia. Si falla (por ejemplo,
// Postgres no responde), se vuelve a intentar en la siguiente invocación.
func dependencias(ctx context.Context) (*validador, error) {
	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		return global, nil
	}
	v, err := desdeEntorno(ctx)
	if err != nil {
		return nil, err
	}
	global = v
	return v, nil
}

// ValidarCarga es el punto de entrada (entry_point en Terraform).
// Devolver nil confirma el mensaje; devolver error hace que Eventarc lo
// reintente.
func ValidarCarga(ctx context.Context, e event.Event) error {
	jobID, m, err := leerJobID(e)
	if err != nil {
		logger.Error("mensaje inválido, se descarta", "error", err, "message_id", m.Message.MessageID)
		return nil
	}
	v, err := dependencias(ctx)
	if err != nil {
		logger.Error("no se pudo iniciar la función; Eventarc reintentará", "error", err, "job_id", jobID)
		return err
	}
	if v := m.Message.Attributes["contract_version"]; v != "" && v != versionContrato {
		logger.Warn("versión de contrato distinta, se procesa igual", "contract_version", v, "job_id", jobID)
	}
	return v.procesar(ctx, jobID)
}

// desdeEntorno arma las dependencias con las variables que pone Terraform
// (infra/terraform/funcion.tf).
func desdeEntorno(ctx context.Context) (*validador, error) {
	dbURL := os.Getenv("DATABASE_URL")
	bucket := os.Getenv("S3_BUCKET")
	proyecto := os.Getenv("PUBSUB_PROJECT")
	topic := os.Getenv("PUBSUB_TOPIC")
	if dbURL == "" || bucket == "" || proyecto == "" || topic == "" {
		return nil, fmt.Errorf("faltan variables: DATABASE_URL, S3_BUCKET, PUBSUB_PROJECT o PUBSUB_TOPIC")
	}
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL inválida")
	}
	cfg.MaxConns = 2
	if n, err := strconv.Atoi(os.Getenv("DB_MAX_CONNS")); err == nil && n > 0 {
		cfg.MaxConns = int32(n)
	}
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("conectando a Postgres: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	tokens := &tokenMetadatos{url: urlTokenMetadatos, client: &http.Client{Timeout: 5 * time.Second}}
	return &validador{
		db:    &almacenPostgres{db: pool},
		blobs: &bucketGCS{base: "https://storage.googleapis.com", bucket: bucket, client: client, tokens: tokens},
		cola:  nuevaColaPubSub("https://pubsub.googleapis.com", proyecto, topic, &http.Client{Timeout: 30 * time.Second}, tokens),
		log:   logger,
	}, nil
}

// severidad nombra el nivel como lo espera Cloud Logging, para filtrar por
// severity=ERROR igual que con el worker.
func severidad(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
		if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == slog.LevelWarn {
			a.Value = slog.StringValue("WARNING")
		}
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}
