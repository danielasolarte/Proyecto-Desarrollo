package validarcarga

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Prueba las consultas de repositorio.go contra un Postgres real con las
// migraciones del repo. Solo corre si VALIDAR_PG_URL apunta a una base
// desechable (ver README.md); si no, se salta.
func TestRepositorioPostgres(t *testing.T) {
	dsn := os.Getenv("VALIDAR_PG_URL")
	if dsn == "" {
		t.Skip("sin VALIDAR_PG_URL")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := &pgRepo{db: db}

	// Recurso y asset mínimos. Las llaves foráneas (unidad, curso) se
	// saltan con session_replication_role = replica en la base de prueba.
	var recurso, assetID, jobID string
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.QueryRow(ctx, `INSERT INTO resources (unit_id, type, title, position, processing_status)
		VALUES (gen_random_uuid(), 'pdf', 'prueba', 1, 'pending') RETURNING id`).Scan(&recurso))
	must(db.QueryRow(ctx, `INSERT INTO media_assets (resource_id, kind, original_storage_key, checksum_sha256)
		VALUES ($1, 'pdf', 'resources/x/original', 'declarado') RETURNING id`, recurso).Scan(&assetID))
	must(db.QueryRow(ctx, `INSERT INTO media_jobs (media_asset_id, job_type, status, attempts, max_attempts, idempotency_key)
		VALUES ($1, 'validate_upload', 'pending', 0, 3, $2) RETURNING id`, assetID, assetID+":validate_upload").Scan(&jobID))

	// Dos entregas a la vez: solo una toma el job.
	var wg sync.WaitGroup
	res := make([]error, 2)
	jobs := make([]*job, 2)
	for i := range res {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			jobs[i], res[i] = r.TomarJob(ctx, jobID, time.Minute)
		}(i)
	}
	wg.Wait()
	tomados, bloqueados := 0, 0
	var j *job
	for i, err := range res {
		switch {
		case err == nil:
			tomados++
			j = jobs[i]
		case errors.Is(err, errBloqueado):
			bloqueados++
		default:
			t.Fatalf("TomarJob: %v", err)
		}
	}
	if tomados != 1 || bloqueados != 1 || j.Intentos != 1 || j.Estado != "processing" {
		t.Fatalf("tomados=%d bloqueados=%d job=%+v", tomados, bloqueados, j)
	}
	must(r.RenovarBloqueo(ctx, jobID))

	a, err := r.BuscarAsset(ctx, assetID)
	must(err)
	if a.Kind != "pdf" || a.ResourceID != recurso || a.Checksum == nil || *a.Checksum != "declarado" {
		t.Fatalf("asset = %+v", a)
	}
	must(r.EstadoRecurso(ctx, recurso, "processing"))
	must(r.GuardarValidacion(ctx, assetID, "real", "application/pdf"))

	s1, creado, err := r.CrearJobSiNoExiste(ctx, assetID, tipoEscaneo, assetID+":scan_antivirus")
	must(err)
	s2, creado2, err := r.CrearJobSiNoExiste(ctx, assetID, tipoEscaneo, assetID+":scan_antivirus")
	must(err)
	if !creado || creado2 || s1.ID != s2.ID || s2.Estado != "pending" || s2.MaxIntentos != 3 {
		t.Fatalf("crear dos veces: %v %v %+v %+v", creado, creado2, s1, s2)
	}
	if _, err := r.TomarJob(ctx, s1.ID, time.Minute); !errors.Is(err, errOtroTipo) {
		t.Fatalf("tomar un scan_antivirus = %v, quiero errOtroTipo", err)
	}

	must(r.CerrarJob(ctx, j))
	if _, err := r.TomarJob(ctx, jobID, time.Minute); !errors.Is(err, errTerminado) {
		t.Fatalf("tomar un job done = %v, quiero errTerminado", err)
	}
	if _, err := r.TomarJob(ctx, "00000000-0000-0000-0000-000000000000", time.Minute); !errors.Is(err, errNoExiste) {
		t.Fatalf("job inexistente = %v", err)
	}

	// FallarJob: con intentos vuelve a pending; sin intentos, dead_letter.
	var otro string
	must(db.QueryRow(ctx, `INSERT INTO media_jobs (media_asset_id, job_type, status, attempts, max_attempts, idempotency_key)
		VALUES ($1, 'validate_upload', 'pending', 0, 3, 'otra') RETURNING id`, assetID).Scan(&otro))
	o, err := r.TomarJob(ctx, otro, time.Minute)
	must(err)
	must(r.FallarJob(ctx, o, errors.New("transitorio")))
	if o.Estado != "pending" {
		t.Fatalf("estado = %s", o.Estado)
	}
	o, err = r.TomarJob(ctx, otro, time.Minute)
	must(err)
	o.Intentos = o.MaxIntentos
	must(r.FallarJob(ctx, o, errors.New("rechazo")))

	var estado, ultimo, mime, checksum, recursoEstado string
	var intentos int
	must(db.QueryRow(ctx, `SELECT status, attempts, last_error FROM media_jobs WHERE id = $1`, otro).Scan(&estado, &intentos, &ultimo))
	if estado != "dead_letter" || intentos != 3 || ultimo != "rechazo" {
		t.Errorf("job fallido = %s/%d/%s", estado, intentos, ultimo)
	}
	must(db.QueryRow(ctx, `SELECT checksum_sha256, original_mime_type FROM media_assets WHERE id = $1`, assetID).Scan(&checksum, &mime))
	must(db.QueryRow(ctx, `SELECT processing_status FROM resources WHERE id = $1`, recurso).Scan(&recursoEstado))
	if checksum != "real" || mime != "application/pdf" || recursoEstado != "processing" {
		t.Errorf("asset=%s/%s recurso=%s", checksum, mime, recursoEstado)
	}
}
