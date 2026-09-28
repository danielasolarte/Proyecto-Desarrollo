package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// RecommendedPartSizeBytes es lo que le sugerimos al cliente usar como
// tamaño de cada parte. S3/MinIO exige un mínimo de 5MB para cualquier
// parte que no sea la última; 8MB da margen cómodo.
const RecommendedPartSizeBytes = 8 * 1024 * 1024

// sessionLifetime es cuánto puede tardar el profesor en terminar de subir
// (sección 5.1, punto 5: "reanudable durante 24 horas").
const sessionLifetime = 24 * time.Hour

type InitiateUploadInput struct {
	ResourceID             string
	ResourceKind           domain.MediaKind
	InitiatedBy            string
	ExpectedMimeType       string
	ExpectedSizeBytes      int64
	DeclaredChecksumSHA256 string
}

var validMediaKinds = map[domain.MediaKind]bool{
	domain.MediaKindImage: true, domain.MediaKindVideo: true, domain.MediaKindAudio: true,
	domain.MediaKindPDF: true, domain.MediaKindPresentation: true, domain.MediaKindFile: true,
}

// InitiateUpload abre una carga multipart y guarda el upload_id que
// devuelve el storage. NO devuelve una URL de subida: el cliente pide la
// URL de cada parte por separado con GetUploadPartURL, y puede volver a
// pedirlas si se corta la conexión — eso es lo que hace la carga
// reanudable en vez de un solo PUT gigante.
func InitiateUpload(repo domain.MediaRepository, storage domain.ObjectStorage, in InitiateUploadInput) (*domain.UploadSession, error) {
	if !validMediaKinds[in.ResourceKind] {
		return nil, errors.Join(domain.ErrValidation, errors.New("tipo de recurso no admite carga multimedia"))
	}
	if strings.TrimSpace(in.InitiatedBy) == "" {
		return nil, errors.Join(domain.ErrValidation, errors.New("falta el usuario que inicia la carga"))
	}

	// Cada intento de carga usa una clave de objeto distinta (aunque sea
	// el mismo recurso) para no pisar un archivo que ya quedó listo si el
	// profesor decide volver a subir el original.
	storageKey := fmt.Sprintf("resources/%s/original/%s", in.ResourceID, uuid.NewString())

	uploadID, err := storage.CreateMultipartUpload(storageKey, in.ExpectedMimeType)
	if err != nil {
		return nil, err
	}

	session := &domain.UploadSession{
		ResourceID:      in.ResourceID,
		InitiatedBy:      in.InitiatedBy,
		StorageKey:      storageKey,
		StorageUploadID: &uploadID,
		Status:          domain.UploadUploading,
	}
	if in.ExpectedMimeType != "" {
		session.ExpectedMimeType = &in.ExpectedMimeType
	}
	if in.ExpectedSizeBytes > 0 {
		session.ExpectedSizeBytes = &in.ExpectedSizeBytes
	}
	if in.DeclaredChecksumSHA256 != "" {
		session.DeclaredChecksumSHA256 = &in.DeclaredChecksumSHA256
	}

	if err := repo.CreateUploadSession(session); err != nil {
		// Si Postgres falla después de abrir el multipart en S3, no lo
		// dejamos huérfano.
		_ = storage.AbortMultipartUpload(storageKey, uploadID)
		return nil, err
	}

	return session, nil
}

// sessionGuard centraliza las validaciones que se repiten en cada
// operación sobre una sesión de carga en curso (usada por
// GetUploadPartURL, ListUploadedParts y CompleteUpload).
func sessionGuard(repo domain.MediaRepository, sessionID string, requestedBy string, isAdmin bool) (*domain.UploadSession, error) {
	session, err := repo.FindUploadSessionByID(sessionID)
	if err != nil {
		return nil, err
	}
	if session.InitiatedBy != requestedBy && !isAdmin {
		return nil, domain.ErrForbidden
	}
	if session.Status != domain.UploadInitiated && session.Status != domain.UploadUploading {
		return nil, domain.ErrUploadNotActive
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, domain.ErrUploadExpired
	}
	return session, nil
}