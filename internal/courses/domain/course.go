// Package domain contiene los modelos y las reglas de forma del módulo de
// cursos: la jerarquía Curso -> Versión -> Módulo -> Unidad -> Recurso.
// Este paquete no importa nada de HTTP ni de Postgres: eso es justamente
// lo que exige la sección 7 del enunciado sobre desacoplar el dominio.
package domain

import (
	"errors"
	"time"
)

// ---------- enums ----------

type VersionStatus string

const (
	VersionStatusDraft      VersionStatus = "draft"
	VersionStatusPublished  VersionStatus = "published"
	VersionStatusSuperseded VersionStatus = "superseded"
)

type ChangeType string

const (
	ChangeTypeMinor ChangeType = "minor"
	ChangeTypeMajor ChangeType = "major"
)

type ResourceType string

const (
	ResourceRichText     ResourceType = "rich_text"
	ResourceImage        ResourceType = "image"
	ResourceVideo        ResourceType = "video"
	ResourceAudio        ResourceType = "audio"
	ResourcePDF          ResourceType = "pdf"
	ResourcePresentation ResourceType = "presentation"
	ResourceFile         ResourceType = "file"
	ResourceIframe       ResourceType = "iframe"
	ResourceLink         ResourceType = "link"
	ResourceQuiz         ResourceType = "quiz"
)

// mediaResourceTypes son los tipos cuyo contenido depende del módulo de
// multimedia (Persona C) y por lo tanto tienen processing_status.
var mediaResourceTypes = map[ResourceType]bool{
	ResourceImage:        true,
	ResourceVideo:        true,
	ResourceAudio:        true,
	ResourcePDF:          true,
	ResourcePresentation: true,
	ResourceFile:         true,
}

func (t ResourceType) IsMedia() bool { return mediaResourceTypes[t] }

type ProcessingStatus string

const (
	ProcessingPending    ProcessingStatus = "pending"
	ProcessingProcessing ProcessingStatus = "processing"
	ProcessingReady      ProcessingStatus = "ready"
	ProcessingFailed     ProcessingStatus = "failed"
)

// ---------- entidades ----------

type Course struct {
	ID                   string
	TeacherID            string
	Slug                 string
	PublishedVersionID   *string
	LatestDraftVersionID *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CourseVersion struct {
	ID            string
	CourseID      string
	VersionNumber int
	Status        VersionStatus
	Title         string
	Summary       string
	Category      string
	ChangeType    *ChangeType
	PublishedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Module struct {
	ID              string
	CourseVersionID string
	StableID        string
	Title           string
	Position        int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Unit struct {
	ID        string
	ModuleID  string
	StableID  string
	Title     string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Resource struct {
	ID                   string
	UnitID               string
	StableID             string
	Type                 ResourceType
	Title                string
	Position             int
	Visible              bool
	Required             bool
	Downloadable         bool
	ProcessingStatus     *ProcessingStatus
	ContentMarkdown      *string
	ContentDraftMarkdown *string
	DraftSavedAt         *time.Time
	ExternalURL          *string
	MediaAssetID         *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// CourseTree es el árbol completo de una versión: módulos, con sus
// unidades, con sus recursos, ya ordenados por posición. Es lo que se
// devuelve en la previsualización y en la consulta de una versión.
type CourseTree struct {
	Version *CourseVersion
	Modules []ModuleTree
}

type ModuleTree struct {
	Module *Module
	Units  []UnitTree
}

type UnitTree struct {
	Unit      *Unit
	Resources []Resource
}

// ---------- errores de dominio ----------

var (
	ErrNotFound              = errors.New("recurso no encontrado")
	ErrForbidden             = errors.New("no autorizado sobre este recurso")
	ErrVersionNotDraft       = errors.New("la version no esta en borrador")
	ErrCourseHasNoPublished  = errors.New("el curso no tiene version publicada")
	ErrCourseAlreadyHasDraft = errors.New("el curso ya tiene un borrador de actualizacion en curso")
	ErrValidation            = errors.New("error de validacion")
	ErrNotRichText           = errors.New("el recurso no es de tipo rich_text")
)

// ---------- repositorios (implementados en internal/courses/postgres) ----------

type CourseRepository interface {
	CreateCourseWithFirstVersion(course *Course, version *CourseVersion) error
	FindCourseByID(id string) (*Course, error)
	FindCourseBySlug(slug string) (*Course, error)
	UpdateCoursePublishedVersion(courseID string, publishedVersionID *string, latestDraftVersionID *string) error

	FindVersionByID(id string) (*CourseVersion, error)
	FindPublishedVersion(courseID string) (*CourseVersion, error)
	FindDraftVersion(courseID string) (*CourseVersion, error)
	UpdateVersionMetadata(v *CourseVersion) error
	MarkVersionPublished(versionID string, publishedAt time.Time) error
	MarkVersionSuperseded(versionID string) error
	CreateVersion(v *CourseVersion) error
	NextVersionNumber(courseID string) (int, error)

	CreateModule(m *Module) error
	FindModuleByID(id string) (*Module, error)
	ListModulesByVersion(versionID string) ([]Module, error)
	UpdateModule(m *Module) error
	DeleteModule(id string) error

	CreateUnit(u *Unit) error
	FindUnitByID(id string) (*Unit, error)
	ListUnitsByModule(moduleID string) ([]Unit, error)
	UpdateUnit(u *Unit) error
	DeleteUnit(id string) error

	CreateResource(r *Resource) error
	FindResourceByID(id string) (*Resource, error)
	ListResourcesByUnit(unitID string) ([]Resource, error)
	UpdateResource(r *Resource) error
	DeleteResource(id string) error

	// LoadTree carga el árbol completo (módulos, unidades, recursos) de una
	// versión en pocas queries, para la previsualización y para copiar una
	// versión completa al crear un borrador de actualización.
	LoadTree(versionID string) (*CourseTree, error)
}
