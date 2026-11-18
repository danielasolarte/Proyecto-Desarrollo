// Package validarcarga es la Cloud Function mooc-validar-carga (Entrega 5).
//
// ESQUELETO (Daniela): deja fijo el punto de entrada que espera Terraform
// (entry_point = "ValidarCarga") y el contrato. La lógica la implementa
// Mateo siguiendo docs/entrega5/contrato-validacion.md:
//
//  1. Leer job_id del mensaje (mismo contrato que la E4: data = {"job_id"},
//     atributos job_type = validate_upload y contract_version = "1").
//  2. Tomar el job con el mismo UPDATE atómico de ClaimJob. Si otro lo
//     tiene o ya está done, confirmar sin hacer nada (no hay duplicados).
//  3. Leer el original del bucket por partes: SHA-256, MIME real y firma
//     ZIP en presentaciones. Si falla, borrar el original, job en
//     dead_letter, recurso en failed y confirmar (no reintentar).
//  4. Si pasa: guardar checksum y MIME reales en media_assets, cerrar el
//     job como done y encolar scan_antivirus con la clave
//     "<asset_id>:scan_antivirus" en mooc-media-jobs.
//  5. Error transitorio (BD, Storage, Pub/Sub): devolver error para que
//     Eventarc reintente; al tercer intento, dead_letter y confirmar.
//
// Mientras Terraform tenga funcion_validar = false, este código no se
// despliega.
package validarcarga

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
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

type mensajeTrabajo struct {
	JobID string `json:"job_id"`
}

// ValidarCarga recibe un validate_upload. Devolver nil confirma el
// mensaje; devolver error hace que Eventarc lo reintente.
func ValidarCarga(ctx context.Context, e event.Event) error {
	var m mensajePubSub
	if err := e.DataAs(&m); err != nil {
		slog.Error("evento inválido, se descarta", "error", err)
		return nil
	}
	var t mensajeTrabajo
	if err := json.Unmarshal(m.Message.Data, &t); err != nil || t.JobID == "" {
		slog.Error("mensaje sin job_id, se descarta", "message_id", m.Message.MessageID)
		return nil
	}
	slog.Info("validate_upload recibido", "job_id", t.JobID, "message_id", m.Message.MessageID)

	// TODO(Mateo): pasos 2 a 5 del contrato.
	return errors.New("validar-carga: sin implementar")
}
