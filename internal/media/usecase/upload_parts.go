package usecase

import (
	"errors"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

const partUploadURLExpiry = 1 * time.Hour

// GetUploadPartURL devuelve la URL prefirmada para subir una parte
// específica. El cliente puede pedirla varias veces para la misma parte
// (por ejemplo si la URL anterior expiró) sin que eso cuente como una
// parte nueva: quien decide qué partes existen de verdad es S3/MinIO, no
// esta llamada.
func GetUploadPartURL(repo domain.MediaRepository, storage domain.ObjectStorage, sessionID string, partNumber int, requestedBy string, isAdmin bool) (string, error) {
	if partNumber < 1 {
		return "", errors.Join(domain.ErrValidation, errors.New("partNumber debe ser >= 1"))
	}
	session, err := sessionGuard(repo, sessionID, requestedBy, isAdmin)
	if err != nil {
		return "", err
	}
	return storage.PresignUploadPart(session.StorageKey, *session.StorageUploadID, partNumber, partUploadURLExpiry)
}

// ListUploadedParts es lo que el cliente llama al reanudar una carga
// interrumpida: le dice exactamente qué partes ya llegaron completas al
// bucket, para que solo tenga que volver a subir las que faltan.
func ListUploadedParts(repo domain.MediaRepository, storage domain.ObjectStorage, sessionID string, requestedBy string, isAdmin bool) ([]domain.UploadedPart, error) {
	session, err := sessionGuard(repo, sessionID, requestedBy, isAdmin)
	if err != nil {
		return nil, err
	}
	return storage.ListUploadedParts(session.StorageKey, *session.StorageUploadID)
}