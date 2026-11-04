package platform

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Contrato del mensaje de trabajos multimedia (Entrega 4).
//
// El backend publica y el worker consume este mensaje; los dos usan estas
// mismas definiciones para que el formato no se desalinee. Documentado en
// docs/entrega4/contrato-mensaje.md.
//
//   - data: JSON {"job_id": "<uuid>"} (Pub/Sub lo transporta en base64).
//     Solo viaja el ID: el worker relee el media_job de Postgres, que es la
//     fuente de verdad, igual que con asynq en la E3.
//   - attributes: job_type (scan_antivirus, transcode_hls,
//     convert_presentation_to_pdf) y contract_version ("1"). Sirven para
//     filtrar y para los registros; el worker no decide nada solo con ellos.
//
// Cambios compatibles: agregar campos opcionales a data o atributos nuevos.
// Un cambio incompatible sube contract_version y el worker acepta las dos
// versiones mientras haya mensajes viejos en la suscripción.

// ContractVersion es la versión actual del contrato del mensaje.
const ContractVersion = "1"

// Nombres de los atributos del mensaje.
const (
	AttrJobType         = "job_type"
	AttrContractVersion = "contract_version"
)

// JobMessage es el cuerpo (data) del mensaje.
type JobMessage struct {
	JobID string `json:"job_id"`
}

// PushEnvelope es el cuerpo HTTP que Pub/Sub envía al endpoint de una
// suscripción push (POST /pubsub/push en el worker).
type PushEnvelope struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
	// DeliveryAttempt solo viene cuando la suscripción tiene tópico de
	// fallidos (dead letter); cuenta desde 1.
	DeliveryAttempt int `json:"deliveryAttempt"`
}

// ErrInvalidMessage indica un mensaje que nunca se podrá procesar (cuerpo
// mal formado, sin job_id). El worker lo confirma (2xx) en vez de dejar que
// Pub/Sub lo reintente, y lo registra.
var ErrInvalidMessage = errors.New("mensaje de trabajo inválido")

// DecodePushEnvelope lee el sobre de Pub/Sub y devuelve el job_id junto con
// el sobre completo (para los registros: messageId, intento, atributos).
func DecodePushEnvelope(body []byte) (JobMessage, PushEnvelope, error) {
	var env PushEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return JobMessage{}, env, fmt.Errorf("%w: sobre: %v", ErrInvalidMessage, err)
	}
	raw, err := base64.StdEncoding.DecodeString(env.Message.Data)
	if err != nil {
		return JobMessage{}, env, fmt.Errorf("%w: data no es base64: %v", ErrInvalidMessage, err)
	}
	msg, err := DecodeJobMessage(raw)
	return msg, env, err
}

// DecodeJobMessage valida el cuerpo (data) ya decodificado.
func DecodeJobMessage(raw []byte) (JobMessage, error) {
	var msg JobMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return JobMessage{}, fmt.Errorf("%w: data: %v", ErrInvalidMessage, err)
	}
	msg.JobID = strings.TrimSpace(msg.JobID)
	if msg.JobID == "" {
		return JobMessage{}, fmt.Errorf("%w: falta job_id", ErrInvalidMessage)
	}
	return msg, nil
}
