package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

// uploadURLExpiry es cuánto vive la URL prefirmada de escritura. La sesión
// en sí (upload_sessions.expires_at) vive 24 horas para permitir carga
// reanudable (sección 5.1, punto 5); la URL firmada puede ser más corta y
// renovarse, pero para el MVP usamos el mismo horizonte por simplicidad.
const uploadURLExpiry = 24 * time.Hour

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

// InitiateUpload crea el upload_session y devuelve la URL prefirmada a la
// que el frontend debe subir el archivo directamente (nunca a través de la
// API, ver sección 4 del enunciado).
func InitiateUpload(repo domain.MediaRepository, storage domain.ObjectStorage, in InitiateUploadInput) (*domain.UploadSession, string, error) {
	if !validMediaKinds[in.ResourceKind] {
		return nil, "", errors.Join(domain.ErrValidation, errors.New("tipo de recurso no admite carga multimedia"))
	}
	if strings.TrimSpace(in.InitiatedBy) == "" {
		return nil, "", errors.Join(domain.ErrValidation, errors.New("falta el usuario que inicia la carga"))
	}

	// Cada intento de carga usa una clave de objeto distinta (aunque sea
	// el mismo recurso) para no pisar un archivo que ya quedó listo si el
	// profesor decide volver a subir el original.
	storageKey := fmt.Sprintf("resources/%s/original/%s", in.ResourceID, uuid.NewString())

	session := &domain.UploadSession{
		ResourceID:  in.ResourceID,
		InitiatedBy: in.InitiatedBy,
		StorageKey:  storageKey,
		Status:      domain.UploadInitiated,
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
		return nil, "", err
	}

	url, err := storage.PresignUpload(storageKey, in.ExpectedMimeType, uploadURLExpiry)
	if err != nil {
		return nil, "", err
	}

	return session, url, nil
}
