package usecase

import (
	"errors"
	"fmt"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

type CompleteUploadInput struct {
	UploadSessionID string
	ResourceKind    domain.MediaKind
	// RequestedBy se compara contra InitiatedBy: solo quien inició la
	// carga (o un admin, verificado por el handler antes de llamar aquí)
	// puede confirmarla.
	RequestedBy string
	IsAdmin     bool
}

// firstJobForKind decide qué trabajo se encola apenas el archivo llega
// íntegro al bucket. El antivirus corre siempre primero; lo que sigue
// depende del tipo de recurso (sección 5.1, puntos 5 y 6).
func firstJobForKind(kind domain.MediaKind) domain.JobType {
	return domain.JobScanAntivirus
}

// CompleteUpload verifica que el archivo llegó completo al bucket
// (StatObject), contrasta el checksum si el cliente declaró uno, cierra la
// sesión de carga, crea o actualiza el media_asset, y encola el primer job
// de procesamiento de forma idempotente: si esta función se llama dos
// veces para la misma sesión (doble entrega), no se crean dos jobs.
func CompleteUpload(repo domain.MediaRepository, storage domain.ObjectStorage, queue domain.JobQueue, in CompleteUploadInput) (*domain.MediaAsset, error) {
	session, err := repo.FindUploadSessionByID(in.UploadSessionID)
	if err != nil {
		return nil, err
	}

	if session.InitiatedBy != in.RequestedBy && !in.IsAdmin {
		return nil, domain.ErrForbidden
	}
	if session.Status == domain.UploadCompleted {
		// Doble entrega: la sesión ya se cerró antes. Devolvemos el asset
		// existente en vez de fallar o duplicar trabajo.
		return repo.FindAssetByResourceID(session.ResourceID)
	}
	if session.Status != domain.UploadInitiated && session.Status != domain.UploadUploading {
		return nil, domain.ErrUploadNotActive
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, domain.ErrUploadExpired
	}

	sizeBytes, checksum, exists, err := storage.StatObject(session.StorageKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.Join(domain.ErrValidation, errors.New("el archivo no aparece en el bucket todavia"))
	}
	if session.DeclaredChecksumSHA256 != nil && *session.DeclaredChecksumSHA256 != checksum {
		return nil, domain.ErrChecksumMismatch
	}

	now := time.Now()
	session.Status = domain.UploadCompleted
	session.CompletedAt = &now
	if err := repo.UpdateUploadSession(session); err != nil {
		return nil, err
	}

	mimeType := ""
	if session.ExpectedMimeType != nil {
		mimeType = *session.ExpectedMimeType
	}

	asset, err := repo.FindAssetByResourceID(session.ResourceID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if asset == nil {
		asset = &domain.MediaAsset{
			ResourceID:         session.ResourceID,
			Kind:               in.ResourceKind,
			OriginalStorageKey: session.StorageKey,
			OriginalMimeType:   &mimeType,
			OriginalSizeBytes:  &sizeBytes,
			ChecksumSHA256:     &checksum,
		}
		if err := repo.CreateAsset(asset); err != nil {
			return nil, err
		}
	} else {
		// Reemplazo de un original ya existente (el profesor volvió a
		// subir el archivo): se actualiza el asset y se limpia cualquier
		// derivado anterior, que quedará obsoleto hasta el reprocesamiento.
		asset.OriginalStorageKey = session.StorageKey
		asset.OriginalMimeType = &mimeType
		asset.OriginalSizeBytes = &sizeBytes
		asset.ChecksumSHA256 = &checksum
		asset.HLSManifestKey = nil
		asset.ConvertedPDFKey = nil
		if err := repo.UpdateAsset(asset); err != nil {
			return nil, err
		}
	}

	jobType := firstJobForKind(in.ResourceKind)
	idempotencyKey := fmt.Sprintf("%s:%s", asset.ID, jobType)
	if _, err := EnqueueJob(repo, queue, asset.ID, jobType, idempotencyKey); err != nil {
		return nil, err
	}

	return asset, nil
}
