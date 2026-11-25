package validarcarga

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

// Reglas de la validación. Son las mismas que tenía el worker hasta la
// Entrega 4 (internal/media/worker/processor.go): el SHA-256 real, el MIME
// real por firma binaria (http.DetectContentType sobre los primeros 512
// bytes) y, para presentaciones, la firma de contenedor ZIP (pptx y odp).

// tiposPermitidos limita el MIME real por tipo de recurso. "presentation"
// no está aquí (DetectContentType no distingue un pptx de un zip
// cualquiera): se exige la firma ZIP. "file" no tiene restricción.
var tiposPermitidos = map[string]map[string]bool{
	"image": {"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true},
	"video": {"video/mp4": true, "video/webm": true, "video/quicktime": true, "video/x-msvideo": true},
	"audio": {"audio/mpeg": true, "audio/wav": true, "audio/ogg": true, "audio/mp4": true, "audio/x-wav": true},
	"pdf":   {"application/pdf": true},
}

var firmaZIP = []byte{0x50, 0x4B, 0x03, 0x04}

// inspeccion es lo que se sabe del archivo después de leerlo una vez.
type inspeccion struct {
	Checksum string
	MIME     string
	EsZIP    bool
	Bytes    int64
}

// inspeccionar lee el archivo en flujo, una sola vez y sin guardarlo
// entero en memoria: calcula el SHA-256 y mira la cabecera.
func inspeccionar(r io.Reader) (inspeccion, error) {
	h := sha256.New()
	cabecera := make([]byte, 512)
	n, err := io.ReadFull(r, cabecera)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return inspeccion{}, err
	}
	cabecera = cabecera[:n]
	h.Write(cabecera)
	resto, err := io.Copy(h, r)
	if err != nil {
		return inspeccion{}, err
	}
	return inspeccion{
		Checksum: hex.EncodeToString(h.Sum(nil)),
		MIME:     http.DetectContentType(cabecera),
		EsZIP:    len(cabecera) >= 4 && bytes.Equal(cabecera[:4], firmaZIP),
		Bytes:    int64(n) + resto,
	}, nil
}

// motivoRechazo devuelve por qué el archivo no pasa, o "" si pasa.
func motivoRechazo(kind string, declarado *string, ins inspeccion) string {
	if declarado != nil && *declarado != "" && *declarado != ins.Checksum {
		return "el checksum declarado no coincide con el archivo recibido: declarado=" + *declarado + " calculado=" + ins.Checksum
	}
	if permitidos, restringido := tiposPermitidos[kind]; restringido && !permitidos[ins.MIME] {
		return "el tipo MIME real no está permitido para este recurso: detectado=" + ins.MIME
	}
	if kind == "presentation" && !ins.EsZIP {
		return "el archivo no tiene firma de contenedor zip (pptx/odp)"
	}
	return ""
}
