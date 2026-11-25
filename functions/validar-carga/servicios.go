package validarcarga

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Cloud Storage y Pub/Sub se usan por su API REST con net/http, como el
// backend (internal/media/platform/pubsubqueue.go). Los clientes oficiales
// de Google ya piden Go 1.26 y el runtime de la función es go125; además
// son decenas de dependencias para tres llamadas. La identidad es
// sa-funcion-validar, con el token del servidor de metadatos: no se usan
// las llaves HMAC del backend.

// almacen es lo que la función necesita del bucket de medios.
type almacen interface {
	Abrir(ctx context.Context, clave string) (io.ReadCloser, error)
	Borrar(ctx context.Context, clave string) error
}

// cola publica un job en mooc-media-jobs con el mismo formato que el
// backend (pubsubqueue.go y pubsub_contract.go).
type cola interface {
	Publicar(ctx context.Context, jobID, tipo string) error
}

type tokens interface {
	Token(ctx context.Context) (string, error)
}

// pedir hace una llamada autenticada. Devuelve la respuesta abierta solo
// si el código es 2xx; si no, la cierra y devuelve un error con el cuerpo.
func pedir(ctx context.Context, cli *http.Client, tk tokens, metodo, u string, cuerpo []byte) (*http.Response, error) {
	var body io.Reader
	if cuerpo != nil {
		body = bytes.NewReader(cuerpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, u, body)
	if err != nil {
		return nil, err
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tk != nil {
		t, err := tk.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("token de la cuenta de servicio: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		resp.Body.Close()
		return resp, &errHTTP{Codigo: resp.StatusCode, Cuerpo: strings.TrimSpace(string(raw))}
	}
	return resp, nil
}

type errHTTP struct {
	Codigo int
	Cuerpo string
}

func (e *errHTTP) Error() string { return fmt.Sprintf("respuesta %d: %s", e.Codigo, e.Cuerpo) }

// ---------- Cloud Storage ----------

type gcsAlmacen struct {
	base   string // https://storage.googleapis.com
	bucket string
	cli    *http.Client
	tk     tokens
}

func (g gcsAlmacen) urlObjeto(clave string) string {
	return fmt.Sprintf("%s/storage/v1/b/%s/o/%s", g.base, url.PathEscape(g.bucket), url.PathEscape(clave))
}

// Abrir devuelve el contenido en flujo (alt=media): quien llama lo lee por
// partes y lo cierra.
func (g gcsAlmacen) Abrir(ctx context.Context, clave string) (io.ReadCloser, error) {
	resp, err := pedir(ctx, g.cli, g.tk, http.MethodGet, g.urlObjeto(clave)+"?alt=media", nil)
	if err != nil {
		return nil, fmt.Errorf("storage: leer %s: %w", clave, err)
	}
	return resp.Body, nil
}

// Borrar no falla si el objeto ya no está: un reintento después de un
// borrado parcial debe poder terminar.
func (g gcsAlmacen) Borrar(ctx context.Context, clave string) error {
	resp, err := pedir(ctx, g.cli, g.tk, http.MethodDelete, g.urlObjeto(clave), nil)
	var eh *errHTTP
	if errors.As(err, &eh) && eh.Codigo == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("storage: borrar %s: %w", clave, err)
	}
	resp.Body.Close()
	return nil
}

// ---------- Pub/Sub ----------

// Mismo contrato que la E4: data = {"job_id"} y los atributos job_type y
// contract_version.
const versionContrato = "1"

type pubsubCola struct {
	url string // .../v1/projects/<p>/topics/<t>:publish
	cli *http.Client
	tk  tokens
}

func (p pubsubCola) Publicar(ctx context.Context, jobID, tipo string) error {
	data, err := json.Marshal(mensajeTrabajo{JobID: jobID})
	if err != nil {
		return err
	}
	cuerpo, err := json.Marshal(map[string]any{"messages": []map[string]any{{
		"data": base64.StdEncoding.EncodeToString(data),
		"attributes": map[string]string{
			"job_type":         tipo,
			"contract_version": versionContrato,
		},
	}}})
	if err != nil {
		return err
	}
	resp, err := pedir(ctx, p.cli, p.tk, http.MethodPost, p.url, cuerpo)
	if err != nil {
		return fmt.Errorf("pubsub: publicar %s: %w", jobID, err)
	}
	defer resp.Body.Close()
	var out struct {
		MessageIDs []string `json:"messageIds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.MessageIDs) == 0 {
		return fmt.Errorf("pubsub: respuesta sin messageIds al publicar %s", jobID)
	}
	return nil
}

// ---------- token de la cuenta de servicio ----------

// metadatos pide el token de sa-funcion-validar al servidor de metadatos y
// lo guarda hasta un minuto antes de que venza (igual que el backend).
type metadatos struct {
	cli *http.Client

	mu    sync.Mutex
	token string
	vence time.Time
}

const urlTokenMetadatos = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

func (m *metadatos) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token != "" && time.Now().Before(m.vence) {
		return m.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlTokenMetadatos, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := m.cli.Do(req)
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
	m.vence = time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - time.Minute)
	return m.token, nil
}
