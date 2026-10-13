package domain

import "testing"

func TestCourseStatusFor(t *testing.T) {
	id := "x"
	casos := []struct {
		nombre           string
		publicado, draft *string
		esperado         string
	}{
		{"nuevo, nunca publicado", nil, &id, CourseStatusDraft},
		{"publicado sin cambios", &id, nil, CourseStatusPublished},
		{"publicado con borrador de actualizacion", &id, &id, CourseStatusPublishedWithDraft},
		{"sin versiones (no deberia pasar)", nil, nil, CourseStatusDraft},
	}
	for _, c := range casos {
		if got := CourseStatusFor(c.publicado, c.draft); got != c.esperado {
			t.Errorf("%s: esperaba %q, obtuve %q", c.nombre, c.esperado, got)
		}
	}
}
