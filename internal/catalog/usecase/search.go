package usecase

import "github.com/equipo-mooc/plataforma-mooc/internal/catalog/domain"

type SearchInput struct {
	Query    string
	Category string
	Cursor   string
	Limit    int
}

type SearchResult struct {
	Courses    []domain.CourseSummary
	NextCursor string
}

// SearchCatalog busca únicamente entre cursos publicados: un curso en
// borrador nunca debe aparecer en el catálogo de un estudiante.
func SearchCatalog(repo domain.CatalogRepository, in SearchInput) (*SearchResult, error) {
	courses, next, err := repo.SearchPublishedCourses(domain.SearchFilter{
		Query: in.Query, Category: in.Category, Cursor: in.Cursor, Limit: in.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &SearchResult{Courses: courses, NextCursor: next}, nil
}
