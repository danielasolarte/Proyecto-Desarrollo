package validarcarga

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Reglas de validación de la carga. Son las mismas que tenía el worker en
// scan_antivirus hasta la Entrega 4 (inspectFile, looksLikeZip y
// allowedMimeTypesByKind en internal/media/worker/processor.go); desde la
// Entrega 5 solo viven aquí.

// mimePermitidos es la lista blanca de MIME reales por tipo de recurso.
// http.DetectContentType no distingue un .pptx o .odp de un .zip
// cualquiera, así que las presentaciones no se validan con esta lista sino
// con la firma ZIP. "file" no tiene lista: es genérico a propósito.
var mimePermitidos = map[string]map[string]bool{
	"image": {"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true},
	"video": {"video/mp4": true, "video/webm": true, "video/quicktime": true, "video/x-msvideo": true},
	"audio": {"audio/mpeg": true, "audio/wav": true, "audio/ogg": true, "audio/mp4": true, "audio/x-wav": true},
	"pdf":   {"application/pdf": true},
}

var firmaZip = []byte{0x50, 0x4B, 0x03, 0x04} // "PK\x03\x04"

// inspeccion es lo que se sabe del archivo después de leerlo una vez.
type inspeccion struct {
	Checksum string // SHA-256 en hexadecimal
	Mime     string // MIME real por firma binaria
	EsZip    bool   // empieza con PK\x03\x04
}

// inspeccionar lee r una sola vez, en flujo: los primeros 512 bytes sirven
// para el MIME y la firma ZIP, y todo pasa por el SHA-256 sin guardarse en
// memoria ni en disco.
func inspeccionar(r io.Reader) (inspeccion, error) {
	h := sha256.New()
	cabecera := make([]byte, 512)
	n, err := io.ReadFull(r, cabecera)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return inspeccion{}, err
	}
	cabecera = cabecera[:n]
	h.Write(cabecera)
	if _, err := io.Copy(h, r); err != nil {
		return inspeccion{}, err
	}
	return inspeccion{
		Checksum: hex.EncodeToString(h.Sum(nil)),
		Mime:     http.DetectContentType(cabecera),
		EsZip:    bytes.HasPrefix(cabecera, firmaZip),
	}, nil
}

// errRechazo marca un archivo que no pasa la validación. Es un fallo
// permanente: reintentarlo no cambia el contenido.
var errRechazo = errors.New("la carga no pasa la validación")

// revisar aplica las reglas. declarado es el checksum que el cliente mandó
// al iniciar la carga (nil si no mandó ninguno).
func revisar(kind string, declarado *string, in inspeccion) error {
	if declarado != nil && *declarado != in.Checksum {
		return fmt.Errorf("%w: el checksum declarado no coincide con el archivo recibido: declarado=%s calculado=%s",
			errRechazo, *declarado, in.Checksum)
	}
	if permitidos, restringido := mimePermitidos[kind]; restringido && !permitidos[in.Mime] {
		return fmt.Errorf("%w: el tipo MIME real no está permitido para %s: detectado=%s", errRechazo, kind, in.Mime)
	}
	if kind == "presentation" && !in.EsZip {
		return fmt.Errorf("%w: el archivo no tiene firma de contenedor zip (pptx/odp)", errRechazo)
	}
	return nil
}
