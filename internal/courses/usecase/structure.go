package usecase

import (
	"errors"
	"strings"

	"github.com/equipo-mooc/plataforma-mooc/internal/courses/domain"
)

// EnsureOwner valida que userID sea el profesor dueño del curso (o que
// isAdmin sea true). Se usa en todos los endpoints de autoría antes de
// mutar nada, para respetar "control por propiedad" (sección 9 del
// enunciado del proyecto).
func EnsureOwner(course *domain.Course, userID string, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	if course.TeacherID != userID {
		return domain.ErrForbidden
	}
	return nil
}

// ensureVersionIsDraft evita cualquier mutación de estructura sobre una
// versión publicada: la inmutabilidad de las versiones publicadas es un
// requisito Must (sección 5.1, punto 3).
func ensureVersionIsDraft(v *domain.CourseVersion) error {
	if v.Status != domain.VersionStatusDraft {
		return domain.ErrVersionNotDraft
	}
	return nil
}

// ---------- módulos ----------

type CreateModuleInput struct {
	VersionID string
	Title     string
	Position  int
}

func CreateModule(repo domain.CourseRepository, in CreateModuleInput) (*domain.Module, error) {
	version, err := repo.FindVersionByID(in.VersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.Join(domain.ErrValidation, errors.New("el titulo del modulo es obligatorio"))
	}

	m := &domain.Module{
		CourseVersionID: in.VersionID,
		Title:           in.Title,
		Position:        in.Position,
	}
	if err := repo.CreateModule(m); err != nil {
		return nil, err
	}
	return m, nil
}

type UpdateModuleInput struct {
	ModuleID string
	Title    string
	Position int
}

func UpdateModule(repo domain.CourseRepository, in UpdateModuleInput) (*domain.Module, error) {
	m, err := repo.FindModuleByID(in.ModuleID)
	if err != nil {
		return nil, err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) != "" {
		m.Title = in.Title
	}
	m.Position = in.Position
	if err := repo.UpdateModule(m); err != nil {
		return nil, err
	}
	return m, nil
}

func DeleteModule(repo domain.CourseRepository, moduleID string) error {
	m, err := repo.FindModuleByID(moduleID)
	if err != nil {
		return err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return err
	}
	return repo.DeleteModule(moduleID)
}

// ---------- unidades ----------

type CreateUnitInput struct {
	ModuleID string
	Title    string
	Position int
}

func CreateUnit(repo domain.CourseRepository, in CreateUnitInput) (*domain.Unit, error) {
	m, err := repo.FindModuleByID(in.ModuleID)
	if err != nil {
		return nil, err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.Join(domain.ErrValidation, errors.New("el titulo de la unidad es obligatorio"))
	}

	u := &domain.Unit{ModuleID: in.ModuleID, Title: in.Title, Position: in.Position}
	if err := repo.CreateUnit(u); err != nil {
		return nil, err
	}
	return u, nil
}

type UpdateUnitInput struct {
	UnitID   string
	Title    string
	Position int
}

func UpdateUnit(repo domain.CourseRepository, in UpdateUnitInput) (*domain.Unit, error) {
	u, err := repo.FindUnitByID(in.UnitID)
	if err != nil {
		return nil, err
	}
	m, err := repo.FindModuleByID(u.ModuleID)
	if err != nil {
		return nil, err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) != "" {
		u.Title = in.Title
	}
	u.Position = in.Position
	if err := repo.UpdateUnit(u); err != nil {
		return nil, err
	}
	return u, nil
}

func DeleteUnit(repo domain.CourseRepository, unitID string) error {
	u, err := repo.FindUnitByID(unitID)
	if err != nil {
		return err
	}
	m, err := repo.FindModuleByID(u.ModuleID)
	if err != nil {
		return err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return err
	}
	return repo.DeleteUnit(unitID)
}

// ---------- recursos ----------

type CreateResourceInput struct {
	UnitID       string
	Type         domain.ResourceType
	Title        string
	Position     int
	Visible      bool
	Required     bool
	Downloadable bool
	ExternalURL  *string
}

var validResourceTypes = map[domain.ResourceType]bool{
	domain.ResourceRichText: true, domain.ResourceImage: true, domain.ResourceVideo: true,
	domain.ResourceAudio: true, domain.ResourcePDF: true, domain.ResourcePresentation: true,
	domain.ResourceFile: true, domain.ResourceIframe: true, domain.ResourceLink: true, domain.ResourceQuiz: true,
}

func CreateResource(repo domain.CourseRepository, in CreateResourceInput) (*domain.Resource, error) {
	if !validResourceTypes[in.Type] {
		return nil, errors.Join(domain.ErrValidation, errors.New("tipo de recurso invalido"))
	}
	u, err := repo.FindUnitByID(in.UnitID)
	if err != nil {
		return nil, err
	}
	m, err := repo.FindModuleByID(u.ModuleID)
	if err != nil {
		return nil, err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.Join(domain.ErrValidation, errors.New("el titulo del recurso es obligatorio"))
	}

	res := &domain.Resource{
		UnitID: in.UnitID, Type: in.Type, Title: in.Title, Position: in.Position,
		Visible: in.Visible, Required: in.Required, Downloadable: in.Downloadable,
		ExternalURL: in.ExternalURL,
	}
	if in.Type.IsMedia() {
		pending := domain.ProcessingPending
		res.ProcessingStatus = &pending
	}
	if err := repo.CreateResource(res); err != nil {
		return nil, err
	}
	return res, nil
}

type UpdateResourceInput struct {
	ResourceID   string
	Title        string
	Position     int
	Visible      bool
	Required     bool
	Downloadable bool
	ExternalURL  *string
}

func UpdateResource(repo domain.CourseRepository, in UpdateResourceInput) (*domain.Resource, error) {
	res, err := resourceVersionGuard(repo, in.ResourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Title) != "" {
		res.Title = in.Title
	}
	res.Position = in.Position
	res.Visible = in.Visible
	res.Required = in.Required
	res.Downloadable = in.Downloadable
	if in.ExternalURL != nil {
		res.ExternalURL = in.ExternalURL
	}
	if err := repo.UpdateResource(res); err != nil {
		return nil, err
	}
	return res, nil
}

func DeleteResource(repo domain.CourseRepository, resourceID string) error {
	_, err := resourceVersionGuard(repo, resourceID)
	if err != nil {
		return err
	}
	return repo.DeleteResource(resourceID)
}

// resourceVersionGuard carga el recurso y confirma que la versión a la que
// pertenece todavía es un borrador editable.
func resourceVersionGuard(repo domain.CourseRepository, resourceID string) (*domain.Resource, error) {
	res, err := repo.FindResourceByID(resourceID)
	if err != nil {
		return nil, err
	}
	u, err := repo.FindUnitByID(res.UnitID)
	if err != nil {
		return nil, err
	}
	m, err := repo.FindModuleByID(u.ModuleID)
	if err != nil {
		return nil, err
	}
	version, err := repo.FindVersionByID(m.CourseVersionID)
	if err != nil {
		return nil, err
	}
	if err := ensureVersionIsDraft(version); err != nil {
		return nil, err
	}
	return res, nil
}
