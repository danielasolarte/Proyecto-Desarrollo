package postgres

import (
	"time"

	"github.com/equipo-mooc/plataforma-mooc/internal/auth/domain"
)

// PostgresOnlySessionStore cumple domain.SessionStore sin caché aparte
// (Entrega 4). Hasta la E3 las sesiones se copiaban en Redis; desde la E4 la
// única fuente es la tabla sessions de Postgres, que usecase.Authenticate ya
// consulta cuando la caché no tiene la sesión. Get siempre responde
// ErrNotFound, así que cada solicitud valida la sesión en Postgres: un
// cierre o revocación se aplica de inmediato en todas las instancias.
//
// Se quitó Redis porque ninguna medición de la E3 mostró que la caché
// hiciera falta; si una prueba de carga lo justifica, otra implementación
// de esta interfaz (por ejemplo Memorystore) se puede volver a conectar en
// cmd/api sin tocar los casos de uso.
type PostgresOnlySessionStore struct{}

func NewPostgresOnlySessionStore() PostgresOnlySessionStore { return PostgresOnlySessionStore{} }

func (PostgresOnlySessionStore) Save(string, domain.User, time.Time) error { return nil }

func (PostgresOnlySessionStore) Get(string) (*domain.User, error) { return nil, domain.ErrNotFound }

func (PostgresOnlySessionStore) Delete(string) error { return nil }
