package validarcarga

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Storage y Pub/Sub se usan por su API REST con el token de la cuenta de
// servicio de la función (sa-funcion-validar), que da el servidor de
// metadatos. Así la función no necesita las llaves HMAC del backend ni
// bibliotecas de Google.

const urlTokenMetadatos = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

type fuenteToken interface {
	Token(ctx context.Context) (string, error)
}

type tokenMetadatos struct {
	url    string
	client *http.Client

	mu    sync.Mutex
	token string
	vence time.Time
}

func (m *tokenMetadatos) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Now().Before(m.vence) {
		return m.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token de metadatos: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token de metadatos: HTTP %d", resp.StatusCode)
	}
	var cuerpo struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cuerpo); err != nil {
		return "", err
	}
	m.token = cuerpo.AccessToken
	// Se renueva un minuto antes de que venza.
	m.vence = time.Now().Add(time.Duration(cuerpo.ExpiresIn)*time.Second - time.Minute)
	return m.token, nil
}

func pedir(ctx context.Context, c *http.Client, tokens fuenteToken, metodo, u string, cuerpo []byte) (*http.Response, error) {
	var r io.Reader
	if cuerpo != nil {
		r = bytes.NewReader(cuerpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, u, r)
	if err != nil {
		return nil, err
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tokens != nil {
		tok, err := tokens.Token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return c.Do(req)
}

// ---------- Cloud Storage ----------

type bucketGCS struct {
	base   string // https://storage.googleapis.com
	bucket string
	client *http.Client
	tokens fuenteToken
}

func (b *bucketGCS) urlObjeto(clave string) string {
	return fmt.Sprintf("%s/storage/v1/b/%s/o/%s", b.base, url.PathEscape(b.bucket), url.PathEscape(clave))
}

func (b *bucketGCS) Abrir(ctx context.Context, clave string) (io.ReadCloser, error) {
	resp, err := pedir(ctx, b.client, b.tokens, http.MethodGet, b.urlObjeto(clave)+"?alt=media", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("leyendo %s: HTTP %d %s", clave, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp.Body, nil
}

func (b *bucketGCS) Borrar(ctx context.Context, clave string) error {
	resp, err := pedir(ctx, b.client, b.tokens, http.MethodDelete, b.urlObjeto(clave), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 404: ya no estaba (un reintento después de borrarlo).
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("borrando %s: HTTP %d", clave, resp.StatusCode)
	}
	return nil
}

// ---------- Pub/Sub ----------

// Contrato del mensaje de la E4 (internal/media/platform/pubsub_contract.go):
// data = {"job_id": "..."}, atributos job_type y contract_version.
const versionContrato = "1"

type colaPubSub struct {
	urlPublicar string
	client      *http.Client
	tokens      fuenteToken
}

func nuevaColaPubSub(base, proyecto, topic string, c *http.Client, tokens fuenteToken) *colaPubSub {
	return &colaPubSub{
		urlPublicar: fmt.Sprintf("%s/v1/projects/%s/topics/%s:publish", base, proyecto, topic),
		client:      c,
		tokens:      tokens,
	}
}

func (q *colaPubSub) Publicar(ctx context.Context, jobID, tipo string) error {
	data, _ := json.Marshal(map[string]string{"job_id": jobID})
	cuerpo, _ := json.Marshal(map[string]any{
		"messages": []map[string]any{{
			"data":       base64.StdEncoding.EncodeToString(data),
			"attributes": map[string]string{"job_type": tipo, "contract_version": versionContrato},
		}},
	})
	resp, err := pedir(ctx, q.client, q.tokens, http.MethodPost, q.urlPublicar, cuerpo)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("publicando en Pub/Sub: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
