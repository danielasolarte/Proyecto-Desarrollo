package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// repoFalso implementa solo ListCoursesByTeacher; el resto de métodos del
// interfaz quedan en nil y harían panic si el handler los usara.
type repoFalso struct {
	domain.CourseRepository
	teacherRecibido string
	cursorRecibido  string
	limitRecibido   int
	cursos          []domain.TeacherCourse
	siguiente       string
	err             error
}

func (r *repoFalso) ListCoursesByTeacher(teacherID, cursor string, limit int) ([]domain.TeacherCourse, string, error) {
	r.teacherRecibido, r.cursorRecibido, r.limitRecibido = teacherID, cursor, limit
	return r.cursos, r.siguiente, r.err
}

func servidor(repo domain.CourseRepository, usuario *authctx.User) *echo.Echo {
	e := echo.New()
	if usuario != nil {
		u := *usuario
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.SetRequest(c.Request().WithContext(authctx.ContextWithUser(c.Request().Context(), u)))
				return next(c)
			}
		})
	}
	RegisterRoutes(e, NewHandler(repo))
	return e
}

func pedir(e *echo.Echo, ruta string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
	return rec
}

func TestListMyCoursesDevuelveLosCursosDelProfesorDeLaSesion(t *testing.T) {
	draft := "v1"
	repo := &repoFalso{
		cursos: []domain.TeacherCourse{
			{ID: "c1", Slug: "go-basico", Title: "Go basico", Status: domain.CourseStatusDraft, LatestDraftVersionID: &draft},
		},
		siguiente: "cursor-2",
	}
	e := servidor(repo, &authctx.User{ID: "profe-1", Role: authctx.RoleTeacher})

	rec := pedir(e, "/api/v1/me/courses?limit=5&cursor=abc")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}
	if repo.teacherRecibido != "profe-1" || repo.cursorRecibido != "abc" || repo.limitRecibido != 5 {
		t.Fatalf("parametros al repositorio: teacher=%q cursor=%q limit=%d", repo.teacherRecibido, repo.cursorRecibido, repo.limitRecibido)
	}
	var cuerpo struct {
		Courses []struct {
			ID                   string `json:"id"`
			Title                string `json:"title"`
			Status               string `json:"status"`
			LatestDraftVersionID string `json:"latest_draft_version_id"`
		} `json:"courses"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatal(err)
	}
	if len(cuerpo.Courses) != 1 || cuerpo.Courses[0].Status != "draft" || cuerpo.Courses[0].LatestDraftVersionID != "v1" || cuerpo.NextCursor != "cursor-2" {
		t.Fatalf("cuerpo inesperado: %s", rec.Body.String())
	}
}

func TestListMyCoursesIgnoraUnTeacherIdEnLaPeticion(t *testing.T) {
	repo := &repoFalso{}
	e := servidor(repo, &authctx.User{ID: "profe-1", Role: authctx.RoleTeacher})
	pedir(e, "/api/v1/me/courses?teacher_id=profe-2")
	if repo.teacherRecibido != "profe-1" {
		t.Fatalf("debe usar el usuario de la sesion, uso %q", repo.teacherRecibido)
	}
}

func TestListMyCoursesLimiteDefectoYMaximo(t *testing.T) {
	repo := &repoFalso{}
	e := servidor(repo, &authctx.User{ID: "p", Role: authctx.RoleTeacher})
	pedir(e, "/api/v1/me/courses")
	if repo.limitRecibido != 20 {
		t.Fatalf("limite por defecto: %d", repo.limitRecibido)
	}
	pedir(e, "/api/v1/me/courses?limit=1000")
	if repo.limitRecibido != 100 {
		t.Fatalf("limite maximo: %d", repo.limitRecibido)
	}
}

func TestListMyCoursesListaVaciaEsArregloVacio(t *testing.T) {
	repo := &repoFalso{cursos: []domain.TeacherCourse{}}
	e := servidor(repo, &authctx.User{ID: "p", Role: authctx.RoleTeacher})
	rec := pedir(e, "/api/v1/me/courses")
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"courses\":[],\"next_cursor\":\"\"}\n" {
		t.Fatalf("esperaba lista vacia, obtuve %d %s", rec.Code, rec.Body.String())
	}
}

func TestListMyCoursesControlDeAcceso(t *testing.T) {
	casos := []struct {
		nombre   string
		usuario  *authctx.User
		esperado int
	}{
		{"sin sesion", nil, http.StatusUnauthorized},
		{"estudiante", &authctx.User{ID: "e", Role: authctx.RoleStudent}, http.StatusForbidden},
		{"profesor", &authctx.User{ID: "p", Role: authctx.RoleTeacher}, http.StatusOK},
		{"admin", &authctx.User{ID: "a", Role: authctx.RoleAdmin}, http.StatusOK},
	}
	for _, c := range casos {
		rec := pedir(servidor(&repoFalso{}, c.usuario), "/api/v1/me/courses")
		if rec.Code != c.esperado {
			t.Errorf("%s: esperaba %d, obtuve %d", c.nombre, c.esperado, rec.Code)
		}
	}
}

func TestListMyCoursesCursorInvalidoEs400(t *testing.T) {
	repo := &repoFalso{err: fmt.Errorf("%w: cursor invalido", domain.ErrValidation)}
	rec := pedir(servidor(repo, &authctx.User{ID: "p", Role: authctx.RoleTeacher}), "/api/v1/me/courses?cursor=@@@")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d: %s", rec.Code, rec.Body.String())
	}
}
