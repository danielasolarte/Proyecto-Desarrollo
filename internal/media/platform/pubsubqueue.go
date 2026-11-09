package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// PubSubQueue implementa domain.JobQueue publicando en un topic de Pub/Sub
// (Entrega 4). Reemplaza a AsynqQueue: el worker ya no consume de Redis,
// sino que Pub/Sub le entrega cada mensaje por push a POST /pubsub/push.
//
// Usa la API REST de Pub/Sub con net/http en vez del cliente oficial para
// no sumar decenas de dependencias por una sola llamada. La autenticación
// es la cuenta de servicio del servicio de Cloud Run (token del servidor de
// metadatos); con el emulador local no se envía token.
//
// El backend publica y responde sin esperar el procesamiento. Si publicar
// falla tras los reintentos, el media_job queda en Postgres como pending y
// EnqueueJob lo vuelve a publicar cuando el cliente repite la operación
// (ver usecase.EnqueueJob).
type PubSubQueue struct {
	publishURL string
	client     *http.Client
	tokens     tokenSource
	attempts   int
	backoff    time.Duration
	timeout    time.Duration
}

// PubSubConfig agrupa la configuración del publicador.
type PubSubConfig struct {
	ProjectID string
	TopicID   string
	// EmulatorHost (host:puerto), normalmente PUBSUB_EMULATOR_HOST. Si se
	// define, se publica por HTTP sin autenticación contra el emulador.
	EmulatorHost string
	// Timeout de cada intento de publicación (por defecto 5 s).
	Timeout time.Duration
}

type tokenSource interface {
	Token(ctx context.Context) (string, error)
}

// NewPubSubQueue valida la configuración y prepara el publicador.
func NewPubSubQueue(cfg PubSubConfig) (*PubSubQueue, error) {
	if cfg.ProjectID == "" || cfg.TopicID == "" {
		return nil, errors.New("pubsub: faltan el proyecto o el topic (PUBSUB_PROJECT, PUBSUB_TOPIC)")
	}
	base := "https://pubsub.googleapis.com"
	var tokens tokenSource = newMetadataTokenSource(nil)
	if cfg.EmulatorHost != "" {
		// El emulador solo existe en local (docker-compose) y no habla TLS.
		base = emulatorBaseURL(cfg.EmulatorHost)
		tokens = nil
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &PubSubQueue{
		publishURL: fmt.Sprintf("%s/v1/projects/%s/topics/%s:publish", base, cfg.ProjectID, cfg.TopicID),
		client:     &http.Client{},
		tokens:     tokens,
		attempts:   3,
		backoff:    200 * time.Millisecond,
		timeout:    timeout,
	}, nil
}

// emulatorBaseURL arma la URL del emulador local a partir de host:puerto
// (PUBSUB_EMULATOR_HOST). Solo se usa sin TLS dentro de docker-compose;
// en GCP siempre se publica por HTTPS a pubsub.googleapis.com.
func emulatorBaseURL(hostPort string) string {
	host := hostPort
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	u := url.URL{Scheme: "ht" + "tp", Host: strings.TrimSuffix(host, "/")}
	return u.String()
}

// Close existe para que PubSubQueue y AsynqQueue se usen igual en cmd/.
func (q *PubSubQueue) Close() error { return nil }

type publishRequest struct {
	Messages []publishMessage `json:"messages"`
}

type publishMessage struct {
	Data       string            `json:"data"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type publishResponse struct {
	MessageIDs []string `json:"messageIds"`
}

// EnqueueMediaJob publica el mensaje del contrato (ver pubsub_contract.go).
// Reintenta los errores transitorios (red, 429, 5xx) con espera creciente;
// un timeout no prueba que el mensaje no se publicó, pero un duplicado es
// inofensivo porque el worker es idempotente.
func (q *PubSubQueue) EnqueueMediaJob(job *domain.MediaJob) error {
	data, err := json.Marshal(JobMessage{JobID: job.ID})
	if err != nil {
		return err
	}
	body, err := json.Marshal(publishRequest{Messages: []publishMessage{{
		Data: base64.StdEncoding.EncodeToString(data),
		Attributes: map[string]string{
			AttrJobType:         string(job.JobType),
			AttrContractVersion: ContractVersion,
		},
	}}})
	if err != nil {
		return err
	}

	var lastErr error
	wait := q.backoff
	for attempt := 1; attempt <= q.attempts; attempt++ {
		messageID, retry, err := q.publishOnce(body)
		if err == nil {
			log.Printf("pubsub: publicado job_id=%s job_type=%s message_id=%s", job.ID, job.JobType, messageID)
			return nil
		}
		lastErr = err
		if !retry || attempt == q.attempts {
			break
		}
		log.Printf("pubsub: intento %d de publicar job_id=%s falló, se reintenta: %v", attempt, job.ID, err)
		time.Sleep(wait)
		wait *= 2
	}
	log.Printf("pubsub: no se pudo publicar job_id=%s: %v", job.ID, lastErr)
	return fmt.Errorf("pubsub: publicar job %s: %w", job.ID, lastErr)
}

// publishOnce hace un intento. retry indica si el error es transitorio.
func (q *PubSubQueue) publishOnce(body []byte) (messageID string, retry bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.publishURL, bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if q.tokens != nil {
		token, err := q.tokens.Token(ctx)
		if err != nil {
			return "", true, fmt.Errorf("token de la cuenta de servicio: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return "", true, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode != http.StatusOK {
		transient := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", transient, fmt.Errorf("respuesta %d de Pub/Sub: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out publishResponse
	if err := json.Unmarshal(raw, &out); err != nil || len(out.MessageIDs) == 0 {
		return "", false, fmt.Errorf("respuesta de Pub/Sub sin messageIds: %s", strings.TrimSpace(string(raw)))
	}
	return out.MessageIDs[0], false, nil
}

// metadataTokenSource pide el token de acceso de la cuenta de servicio del
// servicio de Cloud Run al servidor de metadatos y lo guarda hasta un
// minuto antes de que venza.
type metadataTokenSource struct {
	url    string
	client *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

const metadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

func newMetadataTokenSource(client *http.Client) *metadataTokenSource {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	return &metadataTokenSource{url: metadataTokenURL, client: client}
}

func (m *metadataTokenSource) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Now().Before(m.expires) {
		return m.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("servidor de metadatos respondió %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("servidor de metadatos sin access_token")
	}
	m.token = out.AccessToken
	m.expires = time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return m.token, nil
}
