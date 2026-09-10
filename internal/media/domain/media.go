// Package domain contiene los modelos y las reglas de forma del módulo de
// multimedia: carga reanudable, procesamiento asíncrono (antimalware,
// transcodificación HLS, conversión de presentaciones) e idempotencia de
// workers. Este paquete no importa nada de HTTP, Postgres, Redis/asynq ni
// del SDK de S3: eso es justamente lo que exige la sección 7 del enunciado
// sobre desacoplar el dominio.
package domain

import (
	"errors"
	"time"
)

// ---------- enums ----------

type MediaKind string

const (
	MediaKindImage        MediaKind = "image"
	MediaKindVideo        MediaKind = "video"
	MediaKindAudio        MediaKind = "audio"
	MediaKindPDF          MediaKind = "pdf"
	MediaKindPresentation MediaKind = "presentation"
	MediaKindFile         MediaKind = "file"
)

// UploadStatus refleja el ciclo de vida de una carga multipart directa a
// almacenamiento, reanudable durante 24 horas (sección 5.1, punto 5).
type UploadStatus string

const (
	UploadInitiated UploadStatus = "initiated"
	UploadUploading UploadStatus = "uploading"
	UploadCompleted UploadStatus = "completed"
	UploadAborted   UploadStatus = "aborted"
	UploadExpired   UploadStatus = "expired"
)

// JobType son los tipos de trabajo asíncrono que produce este módulo.
type JobType string

const (
	JobScanAntivirus              JobType = "scan_antivirus"
	JobTranscodeHLS               JobType = "transcode_hls"
	JobConvertPresentationToPDF   JobType = "convert_presentation_to_pdf"
)

// JobStatus refleja el ciclo de vida de un media_job en la cola de asynq.
// dead_letter corresponde a "tras tres reintentos fallidos, el trabajo
// llega a la DLQ y emite una alerta" (sección 6).
type JobStatus string

const (
	JobPending    JobStatus = "pending"
	JobProcessing JobStatus = "processing"
	JobDone       JobStatus = "done"
	JobFailed     JobStatus = "failed"
	JobDeadLetter JobStatus = "dead_letter"
)

// ProcessingStatus espeja internal/courses/domain.ProcessingStatus. Se
// duplica a propósito (son 4 strings) en vez de importar el paquete de
// cursos, para que internal/media no dependa de internal/courses y evitar
// un import circular si en el futuro cursos necesita algo de media.
type ProcessingStatus string

const (
	ProcessingPending    ProcessingStatus = "pending"
	ProcessingProcessing ProcessingStatus = "processing"
	ProcessingReady      ProcessingStatus = "ready"
	ProcessingFailed     ProcessingStatus = "failed"
)

// ---------- entidades ----------

// MediaAsset es el resultado final del procesamiento de un recurso
// multimedia: 1:1 con resources.id (columna resources.media_asset_id del
// módulo de cursos apunta aquí una vez que el asset queda listo).
type MediaAsset struct {
	ID                  string
	ResourceID          string
	Kind                MediaKind
	OriginalStorageKey  string
	OriginalMimeType    *string
	OriginalSizeBytes   *int64
	ChecksumSHA256      *string
	HLSManifestKey      *string // solo video/audio
	DurationSeconds     *int    // solo video/audio
	ConvertedPDFKey     *string // solo presentation
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// UploadSession representa una carga en curso: se crea al pedir la URL
// prefirmada y se cierra al confirmar que el archivo llegó completo.
type UploadSession struct {
	ID                     string
	ResourceID             string
	InitiatedBy            string
	StorageKey             string
	StorageUploadID        *string
	Status                 UploadStatus
	ExpectedMimeType       *string
	ExpectedSizeBytes      *int64
	DeclaredChecksumSHA256 *string
	CreatedAt              time.Time
	ExpiresAt              time.Time
	CompletedAt            *time.Time
	UpdatedAt              time.Time
}

// MediaJob es un trabajo encolado para el worker. La IdempotencyKey evita
// que una doble entrega (mismo evento procesado dos veces) genere salidas
// duplicadas (sección 6, "tolerancia a fallos").
type MediaJob struct {
	ID             string
	MediaAssetID   string
	JobType        JobType
	Status         JobStatus
	Attempts       int
	MaxAttempts    int
	LastError      *string
	IdempotencyKey string
	LockedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---------- errores de dominio ----------

var (
	ErrNotFound            = errors.New("recurso multimedia no encontrado")
	ErrForbidden           = errors.New("no autorizado sobre este recurso multimedia")
	ErrUploadNotActive     = errors.New("la sesion de carga no esta activa")
	ErrUploadExpired       = errors.New("la sesion de carga ha expirado")
	ErrChecksumMismatch    = errors.New("el checksum declarado no coincide con el archivo recibido")
	ErrMimeNotAllowed      = errors.New("el tipo MIME real no esta permitido para este recurso")
	ErrJobAlreadyTerminal  = errors.New("el job ya se encuentra en un estado terminal")
	ErrValidation          = errors.New("error de validacion")
)

// ---------- repositorios (implementados en internal/media/postgres) ----------

type MediaRepository interface {
	CreateAsset(a *MediaAsset) error
	FindAssetByID(id string) (*MediaAsset, error)
	FindAssetByResourceID(resourceID string) (*MediaAsset, error)
	UpdateAsset(a *MediaAsset) error

	CreateUploadSession(s *UploadSession) error
	FindUploadSessionByID(id string) (*UploadSession, error)
	UpdateUploadSession(s *UploadSession) error
	ExpireStaleUploadSessions(olderThan time.Time) (int, error)

	CreateJob(j *MediaJob) error
	FindJobByID(id string) (*MediaJob, error)
	FindJobByIdempotencyKey(key string) (*MediaJob, error)
	UpdateJob(j *MediaJob) error
	ListDeadLetterJobs() ([]MediaJob, error)
}

// ---------- almacenamiento de objetos (implementado en internal/media/platform) ----------
//
// Esta interfaz NO es SQL, por eso vive fuera de postgres/: es la frontera
// hacia MinIO/S3. Gracias a que usecase depende de esta interfaz y no del
// SDK de AWS directamente, se puede testear usecase con un storage falso
// en memoria, igual que se testea con un MediaRepository falso.

type ObjectStorage interface {
	// PresignUpload genera una URL temporal con permiso de escritura para
	// que el frontend suba el archivo directo al bucket, sin pasar por la
	// API (sección 4 del enunciado, regla de arquitectura).
	PresignUpload(key string, contentType string, expiresIn time.Duration) (url string, err error)

	// PresignDownload genera una URL temporal con permiso de lectura,
	// usada por el visor de PDF y el reproductor HLS.
	PresignDownload(key string, expiresIn time.Duration) (url string, err error)

	// StatObject confirma que el objeto ya llegó al bucket tras la carga y
	// devuelve su tamaño y checksum real, para contrastarlos contra lo
	// declarado en el upload_session (sección 6, "procesamiento asíncrono").
	StatObject(key string) (sizeBytes int64, checksumSHA256 string, exists bool, err error)

	// Las siguientes las usa el worker, no la API HTTP: el worker habla
	// directo con MinIO usando credenciales de servidor (no URLs
	// firmadas), para descargar el original, procesarlo localmente y
	// subir los derivados (HLS, PDF convertido).
	DownloadObject(key string, destPath string) error
	UploadObject(sourcePath string, key string, contentType string) error
	DeleteObject(key string) error
}

// ---------- cola de trabajos (implementado en internal/media/platform con asynq) ----------

// JobQueue es la frontera hacia Redis/asynq. CreateJob (MediaRepository)
// solo escribe el registro en Postgres, que es la fuente de verdad; este
// es el paso aparte que efectivamente encola la tarea para que un worker
// la recoja. Se separan porque Postgres y Redis son dos sistemas
// distintos: si el encolado en Redis fallara después de crear el registro
// en Postgres, se necesita poder reintentar solo esa parte.
type JobQueue interface {
	EnqueueMediaJob(job *MediaJob) error
}
