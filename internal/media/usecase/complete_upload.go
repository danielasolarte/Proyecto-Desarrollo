package usecase

import (
	"errors"
	"fmt"

	"github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
)

type CompleteUploadInput struct {
	UploadSessionID string
	ResourceKind    domain.MediaKind
	RequestedBy     string
	IsAdmin         bool
}

// firstJobForKind decide qué trabajo se encola apenas el archivo llega
// íntegro al bucket. El antivirus corre siempre primero, y es también
// donde ahora se verifica el checksum real y el MIME real (ver
// worker/processor.go) — StatObject/ETag ya no alcanza para eso.
func firstJobForKind(kind domain.MediaKind) domain.JobType {
	return domain.JobScanAntivirus
}

// CompleteUpload le pregunta al storage qué partes llegaron de verdad
// (ListUploadedParts, no lo que diga el cliente), las une en el objeto
// final, cierra la sesión, crea/actualiza el media_asset y encola el
// primer job de forma idempotente.
func CompleteUpload(repo domain.MediaRepository, storage domain.ObjectStorage, queue domain.JobQueue, in CompleteUploadInput) (*domain.MediaAsset, error) {
	session, err := repo.FindUploadSessionByID(in.UploadSessionID)
	if err != nil {
		return nil, err
	}
	if session.InitiatedBy != in.RequestedBy && !in.IsAdmin {
		return nil, domain.ErrForbidden
	}
	if session.Status == domain.UploadCompleted {
		// Doble entrega: la sesión ya se cerró antes. Se devuelve el
		// asset existente en vez de fallar o duplicar trabajo.
		return repo.FindAssetByResourceID(session.ResourceID)
	}
	if session.Status != domain.UploadInitiated && session.Status != domain.UploadUploading {
		return nil, domain.ErrUploadNotActive
	}

	parts, err := storage.ListUploadedParts(session.StorageKey, *session.StorageUploadID)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, errors.Join(domain.ErrValidation, errors.New("no se ha subido ninguna parte todavia"))
	}
	if err := storage.CompleteMultipartUpload(session.StorageKey, *session.StorageUploadID, parts); err != nil {
		return nil, err
	}

	sizeBytes, exists, err := storage.StatObject(session.StorageKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.Join(domain.ErrValidation, errors.New("el objeto no aparece en el bucket tras completar el multipart"))
	}

	session.Status = domain.UploadCompleted
	if err := repo.UpdateUploadSession(session); err != nil {
		return nil, err
	}

	mimeType := ""
	if session.ExpectedMimeType != nil {
		mimeType = *session.ExpectedMimeType
	}

	// El checksum declarado se guarda aquí "provisionalmente" en el campo
	// que normalmente lleva el checksum REAL: el worker lo recalcula de
	// verdad al descargar el archivo (ver scanAntivirus en processor.go)
	// y lo contrasta contra este valor antes de sobreescribirlo con el
	// definitivo. Evita necesitar una columna aparte solo para esto.
	declaredChecksum := session.DeclaredChecksumSHA256

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
			ChecksumSHA256:     declaredChecksum,
		}
		if err := repo.CreateAsset(asset); err != nil {
			return nil, err
		}
	} else {
		asset.OriginalStorageKey = session.StorageKey
		asset.OriginalMimeType = &mimeType
		asset.OriginalSizeBytes = &sizeBytes
		asset.ChecksumSHA256 = declaredChecksum
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