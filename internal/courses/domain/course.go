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
	ID                   string    `json:"id"`
	TeacherID            string    `json:"teacher_id"`
	Slug                 string    `json:"slug"`
	PublishedVersionID   *string   `json:"published_version_id,omitempty"`
	LatestDraftVersionID *string   `json:"latest_draft_version_id,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type CourseVersion struct {
	ID            string        `json:"id"`
	CourseID      string        `json:"course_id"`
	VersionNumber int           `json:"version_number"`
	Status        VersionStatus `json:"status"`
	Title         string        `json:"title"`
	Summary       string        `json:"summary"`
	Category      string        `json:"category"`
	ChangeType    *ChangeType   `json:"change_type,omitempty"`
	PublishedAt   *time.Time    `json:"published_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type Module struct {
	ID              string    `json:"id"`
	CourseVersionID string    `json:"course_version_id"`
	StableID        string    `json:"stable_id"`
	Title           string    `json:"title"`
	Position        int       `json:"position"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Unit struct {
	ID        string    `json:"id"`
	ModuleID  string    `json:"module_id"`
	StableID  string    `json:"stable_id"`
	Title     string    `json:"title"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Resource struct {
	ID                   string            `json:"id"`
	UnitID               string            `json:"unit_id"`
	StableID             string            `json:"stable_id"`
	Type                 ResourceType      `json:"type"`
	Title                string            `json:"title"`
	Position             int               `json:"position"`
	Visible              bool              `json:"visible"`
	Required             bool              `json:"required"`
	Downloadable         bool              `json:"downloadable"`
	ProcessingStatus     *ProcessingStatus `json:"processing_status,omitempty"`
	ContentMarkdown      *string           `json:"content_markdown,omitempty"`
	ContentDraftMarkdown *string           `json:"content_draft_markdown,omitempty"`
	DraftSavedAt         *time.Time        `json:"draft_saved_at,omitempty"`
	ExternalURL          *string           `json:"external_url,omitempty"`
	MediaAssetID         *string           `json:"media_asset_id,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// CourseTree es el árbol completo de una versión: módulos, con sus
// unidades, con sus recursos, ya ordenados por posición. Es lo que se
// devuelve en la previsualización y en la consulta de una versión.
type CourseTree struct {
	Version *CourseVersion `json:"version"`
	Modules []ModuleTree   `json:"modules"`
}

type ModuleTree struct {
	Module *Module    `json:"module"`
	Units  []UnitTree `json:"units"`
}

type UnitTree struct {
	Unit      *Unit      `json:"unit"`
	Resources []Resource `json:"resources"`
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
