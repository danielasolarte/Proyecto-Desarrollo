package validarcarga

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

var (
	pngMin = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 100)...)
	zipMin = append([]byte("PK\x03\x04"), bytes.Repeat([]byte{1}, 2000)...)
	pdfMin = []byte("%PDF-1.4\n%contenido de prueba\n")
)

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestInspeccionar(t *testing.T) {
	grande := append(append([]byte{}, zipMin...), bytes.Repeat([]byte("x"), 3<<20)...)
	casos := []struct {
		nombre string
		datos  []byte
		mime   string
		zip    bool
	}{
		{"png", pngMin, "image/png", false},
		{"pdf", pdfMin, "application/pdf", false},
		{"zip grande", grande, "application/zip", true},
		{"vacío", nil, "text/plain; charset=utf-8", false},
		{"menos de 4 bytes", []byte("PK"), "text/plain; charset=utf-8", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in, err := inspeccionar(bytes.NewReader(c.datos))
			if err != nil {
				t.Fatal(err)
			}
			if in.Checksum != sha(c.datos) {
				t.Errorf("checksum = %s, quiero %s", in.Checksum, sha(c.datos))
			}
			if in.Mime != c.mime {
				t.Errorf("mime = %q, quiero %q", in.Mime, c.mime)
			}
			if in.EsZip != c.zip {
				t.Errorf("zip = %v, quiero %v", in.EsZip, c.zip)
			}
		})
	}
}

func TestRevisar(t *testing.T) {
	ok := sha(pngMin)
	malo := "0000"
	casos := []struct {
		nombre    string
		kind      string
		declarado *string
		datos     []byte
		rechaza   bool
	}{
		{"imagen correcta con checksum", "image", &ok, pngMin, false},
		{"imagen sin checksum declarado", "image", nil, pngMin, false},
		{"checksum distinto", "image", &malo, pngMin, true},
		{"pdf que es png", "pdf", nil, pngMin, true},
		{"video que es pdf", "video", nil, pdfMin, true},
		{"presentación zip", "presentation", nil, zipMin, false},
		{"presentación sin firma zip", "presentation", nil, pdfMin, true},
		{"file acepta cualquier cosa", "file", nil, pdfMin, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in, _ := inspeccionar(bytes.NewReader(c.datos))
			err := revisar(c.kind, c.declarado, in)
			if c.rechaza != (err != nil) {
				t.Fatalf("revisar = %v, rechaza esperado %v", err, c.rechaza)
			}
			if err != nil && !errors.Is(err, errRechazo) {
				t.Errorf("el rechazo debe envolver errRechazo: %v", err)
			}
		})
	}
}
