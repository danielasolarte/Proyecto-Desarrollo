package usecase

import (
	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
	"github.com/equipo-mooc/plataforma-mooc/internal/platform/pagination"
)

// ListTeacherCoursesResult es una página de "Mis cursos" del profesor.
type ListTeacherCoursesResult struct {
	Courses    []domain.TeacherCourse
	NextCursor string
}

// ListTeacherCourses lista los cursos propios del profesor autenticado,
// publicados o no (el catálogo solo muestra publicados). Nunca recibe el
// teacherID desde la petición: siempre es el del usuario de la sesión, así
// un profesor no puede listar los cursos de otro.
func ListTeacherCourses(repo domain.CourseRepository, teacherID, cursor string, limit int) (*ListTeacherCoursesResult, error) {
	if teacherID == "" {
		return nil, domain.ErrForbidden
	}
	courses, next, err := repo.ListCoursesByTeacher(teacherID, cursor, pagination.NormalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	return &ListTeacherCoursesResult{Courses: courses, NextCursor: next}, nil
}
