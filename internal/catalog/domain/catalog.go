// Package domain contiene el modelo del catálogo público (búsqueda de
// cursos publicados) y de las inscripciones de estudiantes.
package domain

import (
	"errors"
	"time"
)

type EnrollmentStatus string

const (
	EnrollmentActive    EnrollmentStatus = "active"
	EnrollmentWithdrawn EnrollmentStatus = "withdrawn"
)

// CourseSummary es la fila que se muestra en el catálogo: solo lo que un
// estudiante necesita para decidir si le interesa un curso, sin exponer
// nada del árbol de contenido (eso vive en el módulo de cursos).
type CourseSummary struct {
	CourseID    string
	Slug        string
	Title       string
	Summary     string
	Category    string
	PublishedAt time.Time
}

type Enrollment struct {
	ID          string
	StudentID   string
	CourseID    string
	Status      EnrollmentStatus
	EnrolledAt  time.Time
	WithdrawnAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SearchFilter struct {
	Query    string
	Category string
	Cursor   string // ID del último curso visto, en texto plano (ya decodificado)
	Limit    int
}

var (
	ErrNotFound           = errors.New("recurso no encontrado")
	ErrCourseNotPublished = errors.New("el curso no esta publicado")
	ErrAlreadyEnrolled    = errors.New("ya estas inscrito en este curso")
	ErrNotEnrolled        = errors.New("no estas inscrito en este curso")
)

type CatalogRepository interface {
	SearchPublishedCourses(filter SearchFilter) (results []CourseSummary, nextCursor string, err error)
	FindPublishedCourseByID(courseID string) (*CourseSummary, error)

	FindEnrollment(studentID, courseID string) (*Enrollment, error)
	CreateEnrollment(e *Enrollment) error
	ReactivateEnrollment(id string) error
	WithdrawEnrollment(id string) error
	ListEnrollmentsByStudent(studentID string) ([]Enrollment, error)
}
