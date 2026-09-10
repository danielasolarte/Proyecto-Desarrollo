// Package worker contiene los "processors" que asynq invoca cuando toma un
// media_job de la cola. Cada processor: descarga el original desde
// MinIO/S3, corre la herramienta correspondiente (ClamAV, FFmpeg,
// LibreOffice), sube el resultado, y actualiza tanto media_assets (este
// módulo) como resources.processing_status (módulo de cursos, vía
// coursesRepo). No conoce nada de HTTP ni de asynq directamente: el
// archivo cmd/worker/main.go es quien conecta esto con la cola real.
package worker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	coursesdomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	mediausecase "github.com/equipo-mooc/plataforma-mooc/internal/media/usecase"
)

// allowedMimeTypesByKind es la lista blanca de MIME reales que aceptamos
// por tipo de recurso. NOTA: http.DetectContentType (stdlib) detecta bien
// imagen/video/audio/pdf por firma binaria, pero NO distingue un .pptx o
// .odp de un .zip genérico (ambos son contenedores ZIP) — por eso
// "presentation" no se valida contra esta lista, solo se confirma que
// parece un ZIP. Para detección MIME robusta de verdad, la alternativa es
// una librería de magic numbers (ej. github.com/gabriel-vasile/mimetype),
// que no agregué para no sumar otra dependencia sin que el equipo lo
// decida.
var allowedMimeTypesByKind = map[mediadomain.MediaKind]map[string]bool{
	mediadomain.MediaKindImage: {"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true},
	mediadomain.MediaKindVideo: {"video/mp4": true, "video/webm": true, "video/quicktime": true, "video/x-msvideo": true},
	mediadomain.MediaKindAudio: {"audio/mpeg": true, "audio/wav": true, "audio/ogg": true, "audio/mp4": true, "audio/x-wav": true},
	mediadomain.MediaKindPDF:   {"application/pdf": true},
	// image, file no tienen lista estricta aparte de la de arriba /
	// ninguna: "file" es deliberadamente genérico (descargables varios).
}

type Processor struct {
	mediaRepo   mediadomain.MediaRepository
	coursesRepo coursesdomain.CourseRepository
	storage     mediadomain.ObjectStorage
	queue       mediadomain.JobQueue
	// workDir es un directorio temporal local (montado en el contenedor
	// del worker) donde se descargan los originales antes de procesarlos.
	// FFmpeg y LibreOffice trabajan sobre archivos en disco, no en memoria.
	workDir string
}

func NewProcessor(mediaRepo mediadomain.MediaRepository, coursesRepo coursesdomain.CourseRepository, storage mediadomain.ObjectStorage, queue mediadomain.JobQueue, workDir string) *Processor {
	return &Processor{mediaRepo: mediaRepo, coursesRepo: coursesRepo, storage: storage, queue: queue, workDir: workDir}
}

// HandleJob es el punto de entrada que cmd/worker/main.go conecta al
// handler de asynq para cada tipo de tarea. Encapsula el ciclo de vida
// completo: marcar processing, ejecutar, marcar done/failed, y si se
// agotaron los reintentos, reflejar el fallo en resources.processing_status
// y encadenar el TODO de alerta (sección 6: "emite una alerta").
func (p *Processor) HandleJob(job *mediadomain.MediaJob) error {
	if err := mediausecase.MarkJobProcessing(p.mediaRepo, job); err != nil {
		return err
	}

	asset, err := p.mediaRepo.FindAssetByID(job.MediaAssetID)
	if err != nil {
		_ = mediausecase.MarkJobFailed(p.mediaRepo, job, err)
		return err
	}

	var procErr error
	switch job.JobType {
	case mediadomain.JobScanAntivirus:
		procErr = p.scanAntivirus(job, asset)
	case mediadomain.JobTranscodeHLS:
		procErr = p.transcodeHLS(job, asset)
	case mediadomain.JobConvertPresentationToPDF:
		procErr = p.convertPresentationToPDF(job, asset)
	default:
		procErr = fmt.Errorf("tipo de job desconocido: %s", job.JobType)
	}

	if procErr != nil {
		if markErr := mediausecase.MarkJobFailed(p.mediaRepo, job, procErr); markErr != nil {
			return markErr
		}
		if job.Status == mediadomain.JobDeadLetter {
			_ = p.setResourceProcessingStatus(asset.ResourceID, coursesdomain.ProcessingFailed, nil)
			// TODO: aquí es donde se dispara la alerta de la sección 6
			// ("tras tres reintentos fallidos... emite una alerta"), por
			// ejemplo publicando en el mismo canal que usa observabilidad
			// (OpenTelemetry / logs estructurados que ya use el equipo).
		}
		return procErr
	}

	return mediausecase.MarkJobDone(p.mediaRepo, job)
}

// setResourceProcessingStatus es la única función de este archivo que
// toca el módulo de cursos: actualiza resources.processing_status y,
// opcionalmente, resources.media_asset_id una vez que el asset ya existe.
func (p *Processor) setResourceProcessingStatus(resourceID string, status coursesdomain.ProcessingStatus, mediaAssetID *string) error {
	res, err := p.coursesRepo.FindResourceByID(resourceID)
	if err != nil {
		return err
	}
	res.ProcessingStatus = &status
	if mediaAssetID != nil {
		res.MediaAssetID = mediaAssetID
	}
	return p.coursesRepo.UpdateResource(res)
}

// ---------- scan_antivirus (+ validación real de checksum y MIME) ----------

func (p *Processor) scanAntivirus(job *mediadomain.MediaJob, asset *mediadomain.MediaAsset) error {
	if err := p.setResourceProcessingStatus(asset.ResourceID, coursesdomain.ProcessingProcessing, nil); err != nil {
		return err
	}

	localPath := filepath.Join(p.workDir, job.ID+"-original")
	if err := p.storage.DownloadObject(asset.OriginalStorageKey, localPath); err != nil {
		return fmt.Errorf("descargando original para escaneo: %w", err)
	}
	defer os.Remove(localPath)

	// ---- checksum real + MIME real ----
	//
	// Esto es lo que reemplaza la verificación que NO se puede hacer de
	// forma confiable en el momento de la carga (el ETag de S3/MinIO no
	// es un SHA256 real, ver la advertencia en platform/s3storage.go).
	// Aquí sí: el archivo ya está completo en disco, así que se puede
	// leer byte a byte.
	realChecksum, realMimeType, err := inspectFile(localPath)
	if err != nil {
		return fmt.Errorf("inspeccionando archivo descargado: %w", err)
	}

	if asset.ChecksumSHA256 != nil && *asset.ChecksumSHA256 != realChecksum {
		// El cliente había declarado un checksum al iniciar la carga y no
		// coincide con lo que realmente llegó: fallo permanente, igual
		// que un archivo infectado (no tiene sentido reintentar).
		_ = p.storage.DeleteObject(asset.OriginalStorageKey)
		job.Attempts = job.MaxAttempts
		return fmt.Errorf("%w: declarado=%s calculado=%s", mediadomain.ErrChecksumMismatch, *asset.ChecksumSHA256, realChecksum)
	}

	if allowed, restricted := allowedMimeTypesByKind[asset.Kind]; restricted && !allowed[realMimeType] {
		_ = p.storage.DeleteObject(asset.OriginalStorageKey)
		job.Attempts = job.MaxAttempts
		return fmt.Errorf("%w: detectado=%s", mediadomain.ErrMimeNotAllowed, realMimeType)
	}
	if asset.Kind == mediadomain.MediaKindPresentation && !looksLikeZip(localPath) {
		_ = p.storage.DeleteObject(asset.OriginalStorageKey)
		job.Attempts = job.MaxAttempts
		return fmt.Errorf("%w: el archivo no tiene firma de contenedor zip (pptx/odp)", mediadomain.ErrMimeNotAllowed)
	}

	// Ambas validaciones pasaron: se deja constancia del checksum y MIME
	// REALES en el asset (sobreescribe lo que era solo el valor declarado
	// por el cliente, guardado provisionalmente en CompleteUpload).
	asset.ChecksumSHA256 = &realChecksum
	asset.OriginalMimeType = &realMimeType
	if err := p.mediaRepo.UpdateAsset(asset); err != nil {
		return err
	}

	// clamdscan habla con el daemon clamav ya levantado en docker-compose
	// (ver modulo-cursos-catalogo.md: "docker compose up -d ... clamav").
	cmd := exec.Command("clamdscan", "--no-summary", localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// Código 1 = ClamAV encontró un virus (no es un error de
			// ejecución). Este es un fallo PERMANENTE: no tiene sentido
			// reintentar un archivo infectado, así que se borra del
			// bucket y se fuerza el job a estado terminal sin más
			// reintentos, aunque max_attempts no se haya agotado.
			_ = p.storage.DeleteObject(asset.OriginalStorageKey)
			job.Attempts = job.MaxAttempts // fuerza dead_letter en MarkJobFailed
			return fmt.Errorf("archivo infectado detectado por clamav: %s", string(output))
		}
		return fmt.Errorf("ejecutando clamdscan: %w", err)
	}

	// Limpio: se encola el siguiente paso según el tipo de asset.
	return p.enqueueNextAfterScan(asset)
}

func (p *Processor) enqueueNextAfterScan(asset *mediadomain.MediaAsset) error {
	switch asset.Kind {
	case mediadomain.MediaKindVideo, mediadomain.MediaKindAudio:
		key := fmt.Sprintf("%s:%s", asset.ID, mediadomain.JobTranscodeHLS)
		_, err := mediausecase.EnqueueJob(p.mediaRepo, p.queue, asset.ID, mediadomain.JobTranscodeHLS, key)
		return err
	case mediadomain.MediaKindPresentation:
		key := fmt.Sprintf("%s:%s", asset.ID, mediadomain.JobConvertPresentationToPDF)
		_, err := mediausecase.EnqueueJob(p.mediaRepo, p.queue, asset.ID, mediadomain.JobConvertPresentationToPDF, key)
		return err
	default:
		// pdf, image, file: no hay procesamiento adicional, el original
		// limpio ya es el recurso final.
		return p.setResourceProcessingStatus(asset.ResourceID, coursesdomain.ProcessingReady, &asset.ID)
	}
}

// ---------- transcode_hls ----------

func (p *Processor) transcodeHLS(job *mediadomain.MediaJob, asset *mediadomain.MediaAsset) error {
	localOriginal := filepath.Join(p.workDir, job.ID+"-original")
	if err := p.storage.DownloadObject(asset.OriginalStorageKey, localOriginal); err != nil {
		return fmt.Errorf("descargando original para transcodificar: %w", err)
	}
	defer os.Remove(localOriginal)

	outDir := filepath.Join(p.workDir, job.ID+"-hls")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(outDir)

	manifestPath := filepath.Join(outDir, "playlist.m3u8")

	// Perfil único para el MVP (sin adaptive bitrate multi-resolución):
	// copia el códec de audio a AAC y video a H.264, segmentos de 6s.
	// "sin upscaling" (sección 10.2) se respeta al no forzar una
	// resolución de salida mayor a la de entrada.
	cmd := exec.Command("ffmpeg",
		"-i", localOriginal,
		"-c:v", "h264", "-c:a", "aac",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(outDir, "segment_%03d.ts"),
		manifestPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg fallo: %w: %s", err, string(output))
	}

	duration, err := probeDurationSeconds(localOriginal)
	if err != nil {
		return fmt.Errorf("leyendo duracion con ffprobe: %w", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	manifestKey := fmt.Sprintf("resources/%s/hls/playlist.m3u8", asset.ResourceID)
	for _, entry := range entries {
		localFile := filepath.Join(outDir, entry.Name())
		remoteKey := fmt.Sprintf("resources/%s/hls/%s", asset.ResourceID, entry.Name())
		contentType := "application/vnd.apple.mpegurl"
		if filepath.Ext(entry.Name()) == ".ts" {
			contentType = "video/mp2t"
		}
		if err := p.storage.UploadObject(localFile, remoteKey, contentType); err != nil {
			return fmt.Errorf("subiendo derivado hls %s: %w", entry.Name(), err)
		}
	}

	asset.HLSManifestKey = &manifestKey
	asset.DurationSeconds = &duration
	if err := p.mediaRepo.UpdateAsset(asset); err != nil {
		return err
	}

	return p.setResourceProcessingStatus(asset.ResourceID, coursesdomain.ProcessingReady, &asset.ID)
}

func probeDurationSeconds(localPath string) (int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		localPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	var seconds float64
	if _, err := fmt.Sscanf(string(output), "%f", &seconds); err != nil {
		return 0, err
	}
	return int(seconds), nil
}

// ---------- convert_presentation_to_pdf ----------

func (p *Processor) convertPresentationToPDF(job *mediadomain.MediaJob, asset *mediadomain.MediaAsset) error {
	localOriginal := filepath.Join(p.workDir, job.ID+filepath.Ext(asset.OriginalStorageKey))
	if err := p.storage.DownloadObject(asset.OriginalStorageKey, localOriginal); err != nil {
		return fmt.Errorf("descargando presentacion original: %w", err)
	}
	defer os.Remove(localOriginal)

	outDir := filepath.Join(p.workDir, job.ID+"-pdf")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(outDir)

	// LibreOffice headless convierte pptx/odp a pdf (sección 5.2, alcance
	// opcional). Requiere que la imagen del worker tenga libreoffice
	// instalado.
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", outDir, localOriginal)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("libreoffice fallo: %w: %s", err, string(output))
	}

	base := filepath.Base(localOriginal)
	generatedPDF := filepath.Join(outDir, base[:len(base)-len(filepath.Ext(base))]+".pdf")
	remoteKey := fmt.Sprintf("resources/%s/converted.pdf", asset.ResourceID)
	if err := p.storage.UploadObject(generatedPDF, remoteKey, "application/pdf"); err != nil {
		return fmt.Errorf("subiendo pdf convertido: %w", err)
	}

	return p.setResourceProcessingStatus(asset.ResourceID, coursesdomain.ProcessingReady, &asset.ID)
}

// ---------- validación de contenido real ----------

// inspectFile calcula el SHA256 real del archivo y detecta su MIME real
// por firma binaria (los primeros 512 bytes, que es lo que
// http.DetectContentType necesita). Ambos se hacen en una sola pasada por
// el archivo para no leerlo dos veces.
func inspectFile(localPath string) (checksumHex string, mimeType string, err error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	hasher := sha256.New()
	header := make([]byte, 512)
	n, _ := io.ReadFull(f, header)
	mimeType = http.DetectContentType(header[:n])

	hasher.Write(header[:n])
	if _, err := io.Copy(hasher, f); err != nil {
		return "", "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), mimeType, nil
}

// looksLikeZip confirma la firma binaria "PK\x03\x04" al inicio del
// archivo. pptx y odp son, por dentro, contenedores ZIP — esto no
// confirma que el CONTENIDO sea una presentación válida (para eso haría
// falta abrir el ZIP y revisar [Content_Types].xml), pero descarta de
// entrada un archivo renombrado que ni siquiera es un ZIP.
func looksLikeZip(localPath string) bool {
	f, err := os.Open(localPath)
	if err != nil {
		return false
	}
	defer f.Close()

	sig := make([]byte, 4)
	if _, err := io.ReadFull(f, sig); err != nil {
		return false
	}
	return bytes.Equal(sig, []byte{0x50, 0x4B, 0x03, 0x04})
}