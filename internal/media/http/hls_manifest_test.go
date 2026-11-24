package http

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

const manifiestoWorker = "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:8\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:8.333333,\nsegment_000.ts\n#EXTINF:3.000000,\nsegment_001.ts\n#EXT-X-ENDLIST\n"

func firmaFalsa(key string) (string, error) {
	return "https://bucket.test/" + key + "?X-Amz-Signature=abc&X-Amz-Expires=7200", nil
}

func TestRewriteFirmaCadaSegmentoYConservaLasEtiquetas(t *testing.T) {
	out, err := rewriteHLSManifest([]byte(manifiestoWorker), "resources/r1/hls/playlist.m3u8", firmaFalsa)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	texto := string(out)
	for _, esperado := range []string{
		"#EXTM3U\n#EXT-X-VERSION:3\n",
		"#EXTINF:8.333333,\nhttps://bucket.test/resources/r1/hls/segment_000.ts?X-Amz-Signature=abc&X-Amz-Expires=7200\n",
		"https://bucket.test/resources/r1/hls/segment_001.ts?X-Amz-Signature=abc",
		"#EXT-X-ENDLIST\n",
	} {
		if !strings.Contains(texto, esperado) {
			t.Fatalf("falta %q en el manifiesto:\n%s", esperado, texto)
		}
	}
	if strings.Contains(texto, "\nsegment_") {
		t.Fatalf("quedó un segmento sin firmar:\n%s", texto)
	}
}

func TestRewriteFirmaAtributosURIYRespetaURLAbsolutas(t *testing.T) {
	m := "#EXTM3U\r\n#EXT-X-MAP:URI=\"init.mp4\"\r\n#EXTINF:4,\r\nhttps://cdn.test/a.ts\r\n#EXTINF:4,\r\nsub/b.ts\r\n"
	out, err := rewriteHLSManifest([]byte(m), "resources/r1/hls/playlist.m3u8", firmaFalsa)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	texto := string(out)
	if !strings.Contains(texto, `#EXT-X-MAP:URI="https://bucket.test/resources/r1/hls/init.mp4?X-Amz-Signature=abc&X-Amz-Expires=7200"`) {
		t.Fatalf("no firmó el atributo URI:\n%s", texto)
	}
	if !strings.Contains(texto, "\nhttps://cdn.test/a.ts\n") {
		t.Fatalf("cambió una URL absoluta:\n%s", texto)
	}
	if !strings.Contains(texto, "https://bucket.test/resources/r1/hls/sub/b.ts?") {
		t.Fatalf("no resolvió la subcarpeta:\n%s", texto)
	}
}

func TestRewriteAceptaManifiestoEnLaRaiz(t *testing.T) {
	out, err := rewriteHLSManifest([]byte("#EXTM3U\nseg.ts\n"), "playlist.m3u8", firmaFalsa)
	if err != nil || !strings.Contains(string(out), "https://bucket.test/seg.ts?") {
		t.Fatalf("resultado inesperado: %q, %v", out, err)
	}
}

func TestRewriteRechazaReferenciasFueraDeLaCarpeta(t *testing.T) {
	for _, m := range []string{
		"#EXTM3U\n../../otro/secreto.mp4\n",
		"#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"../llave.bin\"\n",
	} {
		if _, err := rewriteHLSManifest([]byte(m), "resources/r1/hls/playlist.m3u8", firmaFalsa); err == nil {
			t.Fatalf("esperaba error para %q", m)
		}
	}
	if _, err := rewriteHLSManifest([]byte("../x.ts\n"), "playlist.m3u8", firmaFalsa); err == nil {
		t.Fatal("esperaba error para una referencia fuera de la raíz")
	}
}

func TestRewritePropagaElErrorDeLaFirma(t *testing.T) {
	falla := errors.New("sin credenciales")
	_, err := rewriteHLSManifest([]byte(manifiestoWorker), "resources/r1/hls/playlist.m3u8", func(string) (string, error) {
		return "", falla
	})
	if !errors.Is(err, falla) {
		t.Fatalf("esperaba el error de la firma, obtuve %v", err)
	}
}

// almacenFalso simula el bucket: guarda objetos en memoria y firma con un
// formato reconocible. Solo implementa lo que usa signedHLSManifest.
type almacenFalso struct {
	mediadomain.ObjectStorage
	objetos   map[string]string
	vigencias []time.Duration
	falla     error
}

func (a *almacenFalso) DownloadObject(key, dest string) error {
	if a.falla != nil {
		return a.falla
	}
	contenido, ok := a.objetos[key]
	if !ok {
		return mediadomain.ErrNotFound
	}
	return os.WriteFile(dest, []byte(contenido), 0o600)
}

func (a *almacenFalso) PresignDownload(key string, vigencia time.Duration) (string, error) {
	a.vigencias = append(a.vigencias, vigencia)
	return "https://bucket.test/" + key + "?firma=1", nil
}

func TestSignedHLSManifestDescargaYFirma(t *testing.T) {
	almacen := &almacenFalso{objetos: map[string]string{"resources/r1/hls/playlist.m3u8": manifiestoWorker}}
	out, err := signedHLSManifest(almacen, "resources/r1/hls/playlist.m3u8")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !strings.Contains(string(out), "https://bucket.test/resources/r1/hls/segment_001.ts?firma=1") {
		t.Fatalf("manifiesto sin firmar:\n%s", out)
	}
	if len(almacen.vigencias) != 2 || almacen.vigencias[0] != hlsSegmentURLExpiry {
		t.Fatalf("vigencias inesperadas: %v", almacen.vigencias)
	}
}

func TestSignedHLSManifestPropagaErrorDeDescarga(t *testing.T) {
	almacen := &almacenFalso{falla: errors.New("bucket caído")}
	if _, err := signedHLSManifest(almacen, "resources/r1/hls/playlist.m3u8"); err == nil {
		t.Fatal("esperaba error de descarga")
	}
}
