package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/authctx"
)

// repoArbol simula un curso publicado con un borrador de actualización.
type repoArbol struct {
	domain.CourseRepository
	curso  domain.Course
	arbols map[string]*domain.CourseTree
}

func (r *repoArbol) FindCourseByID(id string) (*domain.Course, error) {
	if id != r.curso.ID {
		return nil, domain.ErrNotFound
	}
	c := r.curso
	return &c, nil
}

func (r *repoArbol) LoadTree(versionID string) (*domain.CourseTree, error) {
	t, ok := r.arbols[versionID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func texto(s string) *string { return &s }

func arbol(id, titulo string, status domain.VersionStatus) *domain.CourseTree {
	ahora := time.Now()
	return &domain.CourseTree{
		Version: &domain.CourseVersion{ID: id, Title: titulo, Summary: "resumen", Category: "dev", Status: status, PublishedAt: &ahora},
		Modules: []domain.ModuleTree{{
			Module: &domain.Module{ID: "m1", Title: "Modulo 1"},
			Units: []domain.UnitTree{{
				Unit: &domain.Unit{ID: "u1", Title: "Unidad 1"},
				Resources: []domain.Resource{
					{ID: "r1", Title: "Lectura", Type: domain.ResourceRichText, Visible: true, Required: true, ContentMarkdown: texto("CONTENIDO-SECRETO")},
					{ID: "r2", Title: "Oculto", Type: domain.ResourceLink, Visible: false, ExternalURL: texto("https://oculto")},
				},
			}},
		}},
	}
}

func nuevoRepoArbol(publicado bool) *repoArbol {
	r := &repoArbol{
		curso:  domain.Course{ID: "c1", TeacherID: "profe-1", Slug: "go"},
		arbols: map[string]*domain.CourseTree{"borrador": arbol("borrador", "Titulo borrador", domain.VersionStatusDraft)},
	}
	borrador := "borrador"
	r.curso.LatestDraftVersionID = &borrador
	if publicado {
		pub := "publicada"
		r.curso.PublishedVersionID = &pub
		r.arbols["publicada"] = arbol("publicada", "Titulo publicado", domain.VersionStatusPublished)
	}
	return r
}

func TestOutlineMuestraLaVersionPublicadaSinContenidoNiOcultos(t *testing.T) {
	e := servidor(nuevoRepoArbol(true), &authctx.User{ID: "est-1", Role: authctx.RoleStudent})
	rec := pedir(e, "/api/v1/courses/c1/outline")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d: %s", rec.Code, rec.Body.String())
	}
	cuerpo := rec.Body.String()
	if strings.Contains(cuerpo, "CONTENIDO-SECRETO") || strings.Contains(cuerpo, "Oculto") || strings.Contains(cuerpo, "borrador") {
		t.Fatalf("el temario expone contenido, recursos ocultos o el borrador: %s", cuerpo)
	}
	var o domain.CourseOutline
	if err := json.Unmarshal(rec.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if o.Title != "Titulo publicado" || len(o.Modules) != 1 || len(o.Modules[0].Units[0].Resources) != 1 || !o.Modules[0].Units[0].Resources[0].Required {
		t.Fatalf("temario inesperado: %+v", o)
	}
}

func TestOutlineDeCursoSinPublicarEs404(t *testing.T) {
	rec := pedir(servidor(nuevoRepoArbol(false), &authctx.User{ID: "est-1", Role: authctx.RoleStudent}), "/api/v1/courses/c1/outline")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
}
