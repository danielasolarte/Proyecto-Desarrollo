package http

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// hlsSegmentURLExpiry es la vigencia de las URL firmadas de cada segmento.
// Es más larga que la del manifiesto porque el reproductor pide los
// segmentos a medida que avanza el video, no todos al principio.
const hlsSegmentURLExpiry = 2 * time.Hour

// hlsURIAttr encuentra los atributos URI="..." de etiquetas como
// #EXT-X-MAP o #EXT-X-KEY, que también apuntan a objetos del bucket.
var hlsURIAttr = regexp.MustCompile(`URI="([^"]+)"`)

// rewriteHLSManifest cambia cada referencia relativa del manifiesto
// (segmentos .ts y atributos URI) por una URL firmada del mismo objeto.
//
// El bucket de medios es privado: si el navegador resuelve
// "segment_000.ts" contra la URL firmada del manifiesto, la firma no se
// hereda y el bucket responde 403. Por eso el backend entrega el
// manifiesto con cada segmento ya firmado. Las URL absolutas se dejan
// igual, y una referencia que salga de la carpeta del manifiesto se
// rechaza para no firmar objetos ajenos al video.
func rewriteHLSManifest(manifest []byte, manifestKey string, sign func(key string) (string, error)) ([]byte, error) {
	dir := path.Dir(manifestKey)
	prefix := dir + "/"
	if dir == "." {
		prefix = ""
	}

	resolve := func(ref string) (string, error) {
		if strings.Contains(ref, "://") {
			return ref, nil
		}
		key := path.Clean(path.Join(dir, ref))
		if key == ".." || strings.HasPrefix(key, "../") || !strings.HasPrefix(key, prefix) {
			return "", fmt.Errorf("manifiesto HLS %q: la referencia %q sale de su carpeta", manifestKey, ref)
		}
		return sign(key)
	}

	lines := strings.Split(strings.ReplaceAll(string(manifest), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			var attrErr error
			lines[i] = hlsURIAttr.ReplaceAllStringFunc(raw, func(match string) string {
				signed, err := resolve(hlsURIAttr.FindStringSubmatch(match)[1])
				if err != nil {
					if attrErr == nil {
						attrErr = err
					}
					return match
				}
				return `URI="` + signed + `"`
			})
			if attrErr != nil {
				return nil, attrErr
			}
		default:
			signed, err := resolve(line)
			if err != nil {
				return nil, err
			}
			lines[i] = signed
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// signedHLSManifest descarga el manifiesto del bucket y lo devuelve con
// los segmentos firmados.
func signedHLSManifest(storage mediadomain.ObjectStorage, manifestKey string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "manifiesto-*.m3u8")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)

	if err := storage.DownloadObject(manifestKey, name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return rewriteHLSManifest(data, manifestKey, func(key string) (string, error) {
		return storage.PresignDownload(key, hlsSegmentURLExpiry)
	})
}
