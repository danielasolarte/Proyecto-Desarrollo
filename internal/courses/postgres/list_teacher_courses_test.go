package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

// Prueba de integración contra PostgreSQL real con las migraciones
// aplicadas. Se salta si no se define MOOC_TEST_DATABASE_URL, para no
// romper "go test ./..." en un equipo sin base de datos de pruebas.
//
//	$env:MOOC_TEST_DATABASE_URL="postgres://mooc:mooc@localhost:5432/mooc?sslmode=disable"
//	go test ./internal/courses/postgres/ -run TestListCoursesByTeacher -v
func TestListCoursesByTeacher(t *testing.T) {
	url := os.Getenv("MOOC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MOOC_TEST_DATABASE_URL no definida")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewCourseRepository(db)

	profe := "00000000-0000-0000-0000-0000000000a1"
	otro := "00000000-0000-0000-0000-0000000000b2"
	limpiar := func() {
		_, _ = db.Exec(ctx, `DELETE FROM courses WHERE teacher_id IN ($1, $2)`, profe, otro)
	}
	limpiar()
	defer limpiar()

	crear := func(teacher, slug, titulo string) *domain.Course {
		c := &domain.Course{TeacherID: teacher, Slug: slug + "-" + time.Now().Format("150405.000000")}
		v := &domain.CourseVersion{Title: titulo, Summary: "resumen " + titulo, Category: "dev"}
		if err := repo.CreateCourseWithFirstVersion(c, v); err != nil {
			t.Fatal(err)
		}
		return c
	}
	borrador := crear(profe, "borrador", "Solo borrador")
	publicado := crear(profe, "publicado", "Publicado")
	// Simula la publicación: la versión pasa a published y el curso deja de tener borrador.
	ahora := time.Now()
	if err := repo.MarkVersionPublished(*publicado.LatestDraftVersionID, ahora); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateCoursePublishedVersion(publicado.ID, publicado.LatestDraftVersionID, nil); err != nil {
		t.Fatal(err)
	}
	crear(profe, "tercero", "Tercero")
	crear(otro, "ajeno", "Curso de otro profesor")

	pagina1, siguiente, err := repo.ListCoursesByTeacher(profe, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pagina1) != 2 || siguiente == "" {
		t.Fatalf("pagina 1: %d cursos, siguiente=%q", len(pagina1), siguiente)
	}
	pagina2, siguiente2, err := repo.ListCoursesByTeacher(profe, siguiente, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pagina2) != 1 || siguiente2 != "" {
		t.Fatalf("pagina 2: %d cursos, siguiente=%q", len(pagina2), siguiente2)
	}

	todos := append(pagina1, pagina2...)
	porID := map[string]domain.TeacherCourse{}
	for _, c := range todos {
		porID[c.ID] = c
		if c.Title == "Curso de otro profesor" {
			t.Fatal("aparecio un curso de otro profesor")
		}
	}
	if len(porID) != 3 {
		t.Fatalf("esperaba 3 cursos distintos entre las dos paginas, obtuve %d", len(porID))
	}
	if b := porID[borrador.ID]; b.Status != domain.CourseStatusDraft || b.Title != "Solo borrador" || b.PublishedAt != nil {
		t.Fatalf("borrador inesperado: %+v", b)
	}
	if p := porID[publicado.ID]; p.Status != domain.CourseStatusPublished || p.Title != "Publicado" || p.PublishedAt == nil || p.LatestDraftVersionID != nil {
		t.Fatalf("publicado inesperado: %+v", p)
	}
	if todos[0].Title != "Tercero" {
		t.Fatalf("orden: el mas reciente debe ir primero, fue %q", todos[0].Title)
	}

	if _, _, err := repo.ListCoursesByTeacher(profe, "no-es-base64-%%%", 2); err == nil {
		t.Fatal("un cursor invalido debe dar error de validacion")
	}
}
