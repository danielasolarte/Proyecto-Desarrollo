package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	catalogDomain "github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
	coursesdomain "github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	mediadomain "github.com/equipo-mooc/plataforma-mooc/internal/media/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// cursosFalsos arma la cadena recurso -> unidad -> módulo -> versión ->
// curso de un solo curso del profesor "profe-1".
type cursosFalsos struct{ coursesdomain.CourseRepository }

func (cursosFalsos) FindResourceByID(id string) (*coursesdomain.Resource, error) {
	if id != "r1" && id != "r-pdf" {
		return nil, coursesdomain.ErrNotFound
	}
	return &coursesdomain.Resource{ID: id, UnitID: "u1"}, nil
}
func (cursosFalsos) FindUnitByID(string) (*coursesdomain.Unit, error) {
	return &coursesdomain.Unit{ID: "u1", ModuleID: "m1"}, nil
}
func (cursosFalsos) FindModuleByID(string) (*coursesdomain.Module, error) {
	return &coursesdomain.Module{ID: "m1", CourseVersionID: "v1"}, nil
}
func (cursosFalsos) FindVersionByID(string) (*coursesdomain.CourseVersion, error) {
	return &coursesdomain.CourseVersion{ID: "v1", CourseID: "c1"}, nil
}
func (cursosFalsos) FindCourseByID(string) (*coursesdomain.Course, error) {
	return &coursesdomain.Course{ID: "c1", TeacherID: "profe-1"}, nil
}

// catalogoFalso: solo "est-inscrito" está inscrito y activo en c1.
type catalogoFalso struct{ catalogDomain.CatalogRepository }

func (catalogoFalso) FindEnrollment(studentID, courseID string) (*catalogDomain.Enrollment, error) {
	if studentID == "est-inscrito" && courseID == "c1" {
		return &catalogDomain.Enrollment{StudentID: studentID, CourseID: courseID, Status: catalogDomain.EnrollmentActive}, nil
	}
	return nil, catalogDomain.ErrNotFound
}

// mediosFalsos: r1 es un video con HLS; r-pdf, un PDF convertido.
type mediosFalsos struct{ mediadomain.MediaRepository }

func (mediosFalsos) FindAssetByResourceID(resourceID string) (*mediadomain.MediaAsset, error) {
	switch resourceID {
	case "r1":
		hls := "resources/r1/hls/playlist.m3u8"
		return &mediadomain.MediaAsset{ID: "a1", ResourceID: "r1", Kind: "video", OriginalStorageKey: "resources/r1/original.mp4", HLSManifestKey: &hls}, nil
	case "r-pdf":
		pdf := "resources/r-pdf/convertido.pdf"
		return &mediadomain.MediaAsset{ID: "a2", ResourceID: "r-pdf", Kind: "pdf", OriginalStorageKey: "resources/r-pdf/original.pptx", ConvertedPDFKey: &pdf}, nil
	}
	return nil, mediadomain.ErrNotFound
}

func servidorReproduccion() *echo.Echo {
	almacen := &almacenFalso{objetos: map[string]string{"resources/r1/hls/playlist.m3u8": manifiestoWorker}}
	h := NewHandler(mediosFalsos{}, cursosFalsos{}, catalogoFalso{}, almacen, nil)
	e := echo.New()
	e.Use(authctx.FakeAuthMiddleware())
	RegisterRoutes(e, h)
	return e
}

func pedirComo(e *echo.Echo, ruta, usuario string, rol authctx.Role) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, ruta, nil)
	req.Header.Set("X-User-Id", usuario)
	req.Header.Set("X-User-Role", string(rol))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func urlDeRespuesta(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var cuerpo struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatalf("respuesta no es JSON: %s", rec.Body.String())
	}
	return cuerpo.URL
}

func TestPlaybackDeVideoApuntaAlManifiestoFirmado(t *testing.T) {
	rec := pedirComo(servidorReproduccion(), "/api/v1/resources/r1/playback", "est-inscrito", authctx.RoleStudent)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}
	if got := urlDeRespuesta(t, rec); got != "/api/v1/resources/r1/playback/manifest.m3u8" {
		t.Fatalf("url inesperada: %q", got)
	}
}

func TestPlaybackDePDFSigueSiendoURLFirmada(t *testing.T) {
	rec := pedirComo(servidorReproduccion(), "/api/v1/resources/r-pdf/playback", "profe-1", authctx.RoleTeacher)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}
	if got := urlDeRespuesta(t, rec); got != "https://bucket.test/resources/r-pdf/convertido.pdf?firma=1" {
		t.Fatalf("url inesperada: %q", got)
	}
}

func TestManifiestoSeSirveConLosSegmentosFirmados(t *testing.T) {
	rec := pedirComo(servidorReproduccion(), "/api/v1/resources/r1/playback/manifest.m3u8", "est-inscrito", authctx.RoleStudent)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
		t.Fatalf("Content-Type inesperado: %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control inesperado: %q", cc)
	}
	cuerpo := rec.Body.String()
	if !strings.Contains(cuerpo, "https://bucket.test/resources/r1/hls/segment_000.ts?firma=1") || strings.Contains(cuerpo, "\nsegment_") {
		t.Fatalf("manifiesto sin firmar:\n%s", cuerpo)
	}
}

func TestManifiestoExigeInscripcion(t *testing.T) {
	rec := pedirComo(servidorReproduccion(), "/api/v1/resources/r1/playback/manifest.m3u8", "est-ajeno", authctx.RoleStudent)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperaba 403, obtuve %d: %s", rec.Code, rec.Body.String())
	}
}

func TestManifiestoDeRecursoSinHLSEs404(t *testing.T) {
	rec := pedirComo(servidorReproduccion(), "/api/v1/resources/r-pdf/playback/manifest.m3u8", "profe-1", authctx.RoleTeacher)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d: %s", rec.Code, rec.Body.String())
	}
}

func TestManifiestoSinSesionEs403(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/r1/playback/manifest.m3u8", nil)
	rec := httptest.NewRecorder()
	servidorReproduccion().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperaba 403, obtuve %d: %s", rec.Code, rec.Body.String())
	}
}
