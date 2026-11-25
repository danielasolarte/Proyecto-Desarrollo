package validarcarga

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

var (
	pdfFalso   = append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 2000)...)
	zipFalso   = append([]byte{0x50, 0x4B, 0x03, 0x04}, bytes.Repeat([]byte("z"), 900)...)
	textoPlano = []byte("hola, esto no es un video")
)

func TestInspeccionarCalculaChecksumMIMEyTamano(t *testing.T) {
	ins, err := inspeccionar(bytes.NewReader(pdfFalso))
	if err != nil {
		t.Fatal(err)
	}
	if ins.Checksum != sha(pdfFalso) || ins.MIME != "application/pdf" || ins.Bytes != int64(len(pdfFalso)) || ins.EsZIP {
		t.Fatalf("inspección inesperada: %+v", ins)
	}
}

func TestInspeccionarArchivoCortoYVacio(t *testing.T) {
	ins, err := inspeccionar(strings.NewReader("ab"))
	if err != nil || ins.Bytes != 2 || ins.Checksum != sha([]byte("ab")) {
		t.Fatalf("corto: %+v %v", ins, err)
	}
	ins, err = inspeccionar(strings.NewReader(""))
	if err != nil || ins.Bytes != 0 || ins.EsZIP {
		t.Fatalf("vacío: %+v %v", ins, err)
	}
}

func TestMotivoRechazo(t *testing.T) {
	pdf, _ := inspeccionar(bytes.NewReader(pdfFalso))
	zip, _ := inspeccionar(bytes.NewReader(zipFalso))
	txt, _ := inspeccionar(bytes.NewReader(textoPlano))
	bien, mal := sha(pdfFalso), "0000"
	casos := []struct {
		nombre    string
		kind      string
		declarado *string
		ins       inspeccion
		rechaza   string
	}{
		{"pdf correcto", "pdf", &bien, pdf, ""},
		{"sin checksum declarado", "pdf", nil, pdf, ""},
		{"checksum distinto", "pdf", &mal, pdf, "checksum"},
		{"video que es texto", "video", nil, txt, "MIME"},
		{"presentación zip", "presentation", nil, zip, ""},
		{"presentación sin zip", "presentation", nil, pdf, "zip"},
		{"archivo genérico", "file", nil, txt, ""},
	}
	for _, c := range casos {
		got := motivoRechazo(c.kind, c.declarado, c.ins)
		if (c.rechaza == "") != (got == "") || !strings.Contains(got, c.rechaza) {
			t.Errorf("%s: motivo %q, se esperaba que contuviera %q", c.nombre, got, c.rechaza)
		}
	}
}
