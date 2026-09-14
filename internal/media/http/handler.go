// Package http expone el módulo de multimedia como endpoints REST bajo
// /api/v1, usando Echo. Igual que en internal/courses/http, esta capa solo
// hace: leer la petición, verificar autenticación/propiedad, llamar al
// usecase correspondiente, y traducir el resultado a JSON.
//
// Este handler depende de DOS repositorios: el propio (mediadomain) y el
// de cursos (coursesdomain), porque para autorizar una carga necesita
// saber a qué curso pertenece el recurso y quién es su profesor dueño —
// exactamente el mismo dato que ya usa internal/courses/http.courseForResource.
package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	coursesdomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	mediausecase "github.com/equipo-mooc/plataforma-mooc/internal/media/usecase"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/apierror"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

const presignedDownloadExpiry = 15 * time.Minute

type Handler struct {
	mediaRepo   mediadomain.MediaRepository
	coursesRepo coursesdomain.CourseRepository
	storage     mediadomain.ObjectStorage
	queue       mediadomain.JobQueue
}

func NewHandler(mediaRepo mediadomain.MediaRepository, coursesRepo coursesdomain.CourseRepository, storage mediadomain.ObjectStorage, queue mediadomain.JobQueue) *Handler {
	return &Handler{mediaRepo: mediaRepo, coursesRepo: coursesRepo, storage: storage, queue: queue}
}

// ---------- helpers compartidos ----------

func currentUser(c echo.Context) (authctx.User, error) {
	u, ok := authctx.UserFromContext(c)
	if !ok {
		return authctx.User{}, mediadomain.ErrForbidden
	}
	return u, nil
}

// ownerOfResource replica la cadena resource -> unit -> module -> version
// -> course que ya usa internal/courses/http.courseForResource, para saber
// quién es el profesor dueño antes de autorizar una carga.
func (h *Handler) ownerOfResource(resourceID string) (*coursesdomain.Course, *coursesdomain.Resource, error) {
	res, err := h.coursesRepo.FindResourceByID(resourceID)
	if err != nil {
		return nil, nil, err
	}
	unit, err := h.coursesRepo.FindUnitByID(res.UnitID)
	if err != nil {
		return nil, nil, err
	}
	module, err := h.coursesRepo.FindModuleByID(unit.ModuleID)
	if err != nil {
		return nil, nil, err
	}
	version, err := h.coursesRepo.FindVersionByID(module.CourseVersionID)
	if err != nil {
		return nil, nil, err
	}
	course, err := h.coursesRepo.FindCourseByID(version.CourseID)
	if err != nil {
		return nil, nil, err
	}
	return course, res, nil
}

func ensureOwner(course *coursesdomain.Course, userID string, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	if course.TeacherID != userID {
		return mediadomain.ErrForbidden
	}
	return nil
}

// mediaKindForResource traduce el tipo de recurso del módulo de cursos al
// MediaKind de este módulo. Los valores de texto coinciden a propósito
// ("video", "pdf", etc.), así que la conversión es directa; solo se valida
// que el tipo efectivamente admita carga multimedia.
func mediaKindForResource(res *coursesdomain.Resource) (mediadomain.MediaKind, error) {
	if !res.Type.IsMedia() {
		return "", mediadomain.ErrValidation
	}
	return mediadomain.MediaKind(res.Type), nil
}

// ---------- iniciar carga ----------

type initiateUploadRequest struct {
	MimeType       string `json:"mime_type"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

func (h *Handler) InitiateUpload(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	course, res, err := h.ownerOfResource(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	if err := ensureOwner(course, user.ID, user.Role == authctx.RoleAdmin); err != nil {
		return respondError(c, err)
	}
	kind, err := mediaKindForResource(res)
	if err != nil {
		return respondError(c, err)
	}

	var req initiateUploadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "cuerpo invalido"))
	}

	session, err := mediausecase.InitiateUpload(h.mediaRepo, h.storage, mediausecase.InitiateUploadInput{
		ResourceID:             resourceID,
		ResourceKind:           kind,
		InitiatedBy:            user.ID,
		ExpectedMimeType:       req.MimeType,
		ExpectedSizeBytes:      req.SizeBytes,
		DeclaredChecksumSHA256: req.ChecksumSHA256,
	})
	if err != nil {
		return respondError(c, err)
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"upload_session":         session,
		"recommended_part_bytes": mediausecase.RecommendedPartSizeBytes,
	})
}

// ---------- partes (multipart reanudable) ----------

func (h *Handler) GetUploadPartURL(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	sessionID := c.Param("uploadSessionID")
	partNumber, err := strconv.Atoi(c.Param("partNumber"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, apierror.New(apierror.CodeValidation, "partNumber invalido"))
	}

	url, err := mediausecase.GetUploadPartURL(h.mediaRepo, h.storage, sessionID, partNumber, user.ID, user.Role == authctx.RoleAdmin)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"url": url})
}

// ListUploadedParts es lo que el cliente llama al reanudar: le dice qué
// partes ya llegaron completas, para subir solo las que faltan.
func (h *Handler) ListUploadedParts(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	sessionID := c.Param("uploadSessionID")

	parts, err := mediausecase.ListUploadedParts(h.mediaRepo, h.storage, sessionID, user.ID, user.Role == authctx.RoleAdmin)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"parts": parts})
}

// ---------- confirmar carga ----------

func (h *Handler) CompleteUpload(c echo.Context) error {
	user, err := currentUser(c)
	if err != nil {
		return respondError(c, err)
	}
	sessionID := c.Param("uploadSessionID")

	session, err := h.mediaRepo.FindUploadSessionByID(sessionID)
	if err != nil {
		return respondError(c, err)
	}
	// La verificación fina de "es el mismo usuario que inició la carga (o
	// admin)" ya la hace mediausecase.CompleteUpload; aquí solo
	// necesitamos el tipo de recurso para saber qué asset/job crear.
	_, res, err := h.ownerOfResource(session.ResourceID)
	if err != nil {
		return respondError(c, err)
	}
	kind, err := mediaKindForResource(res)
	if err != nil {
		return respondError(c, err)
	}

	asset, err := mediausecase.CompleteUpload(h.mediaRepo, h.storage, h.queue, mediausecase.CompleteUploadInput{
		UploadSessionID: sessionID,
		ResourceKind:    kind,
		RequestedBy:     user.ID,
		IsAdmin:         user.Role == authctx.RoleAdmin,
	})
	if err != nil {
		return respondError(c, err)
	}

	// El estado visible del recurso (resources.processing_status) lo posee
	// el módulo de cursos. Este módulo solo notifica que hay un asset
	// nuevo procesándose; quien actualiza processing_status a 'processing'
	// es el propio worker cuando toma el primer job (ver worker/processor.go).
	return c.JSON(http.StatusOK, asset)
}

// ---------- estado del asset ----------

func (h *Handler) GetMediaStatus(c echo.Context) error {
	if _, err := currentUser(c); err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	asset, err := h.mediaRepo.FindAssetByResourceID(resourceID)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, asset)
}

// ---------- reproducción / descarga ----------

// GetPlaybackURL devuelve una URL prefirmada de lectura hacia el derivado
// correcto (manifest HLS para video/audio, el propio original para PDF).
// El control de acceso por inscripción ("¿este estudiante puede ver este
// curso?") se aplica en el módulo de catálogo/progreso antes de llegar
// aquí; este endpoint asume que quien llama ya tiene derecho a ver el
// recurso (sección 6, "control de acceso").
func (h *Handler) GetPlaybackURL(c echo.Context) error {
	if _, err := currentUser(c); err != nil {
		return respondError(c, err)
	}
	resourceID := c.Param("resourceID")
	asset, err := h.mediaRepo.FindAssetByResourceID(resourceID)
	if err != nil {
		return respondError(c, err)
	}

	key := asset.OriginalStorageKey
	if asset.HLSManifestKey != nil {
		key = *asset.HLSManifestKey
	} else if asset.ConvertedPDFKey != nil {
		key = *asset.ConvertedPDFKey
	}

	url, err := h.storage.PresignDownload(key, presignedDownloadExpiry)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"url": url})
}