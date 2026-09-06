package usecase

import (
	"errors"

	"github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"
)

// Enroll inscribe a un estudiante en un curso publicado. Si ya existe una
// inscripción retirada, la reactiva en vez de crear una fila nueva, para
// no perder el historial ni el progreso ya asociado a esa inscripción
// (requisito de "reinscripción conservando progreso").
func Enroll(repo domain.CatalogRepository, studentID, courseID string) (*domain.Enrollment, error) {
	if _, err := repo.FindPublishedCourseByID(courseID); err != nil {
		return nil, err
	}

	existing, err := repo.FindEnrollment(studentID, courseID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	if existing != nil {
		if existing.Status == domain.EnrollmentActive {
			return nil, domain.ErrAlreadyEnrolled
		}
		if err := repo.ReactivateEnrollment(existing.ID); err != nil {
			return nil, err
		}
		existing.Status = domain.EnrollmentActive
		return existing, nil
	}

	e := &domain.Enrollment{StudentID: studentID, CourseID: courseID}
	if err := repo.CreateEnrollment(e); err != nil {
		return nil, err
	}
	e.Status = domain.EnrollmentActive
	return e, nil
}

// Withdraw retira a un estudiante de un curso. No borra la fila: la deja
// en estado "withdrawn" para poder reactivarla luego con Enroll.
func Withdraw(repo domain.CatalogRepository, studentID, courseID string) error {
	existing, err := repo.FindEnrollment(studentID, courseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotEnrolled
		}
		return err
	}
	if existing.Status == domain.EnrollmentWithdrawn {
		return domain.ErrNotEnrolled
	}
	return repo.WithdrawEnrollment(existing.ID)
}

func ListMyEnrollments(repo domain.CatalogRepository, studentID string) ([]domain.Enrollment, error) {
	return repo.ListEnrollmentsByStudent(studentID)
}
