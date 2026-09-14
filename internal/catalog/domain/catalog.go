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
	CourseID    string    `json:"course_id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Category    string    `json:"category"`
	PublishedAt time.Time `json:"published_at"`
}

type Enrollment struct {
	ID          string           `json:"id"`
	StudentID   string           `json:"student_id"`
	CourseID    string           `json:"course_id"`
	Status      EnrollmentStatus `json:"status"`
	EnrolledAt  time.Time        `json:"enrolled_at"`
	WithdrawnAt *time.Time       `json:"withdrawn_at,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
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
