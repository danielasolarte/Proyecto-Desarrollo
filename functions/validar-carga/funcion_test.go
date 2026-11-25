package validarcarga

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
)

// ---------- dobles en memoria ----------

type jobMem struct {
	job
	clave    string
	bloqueo  time.Time
	ultError string
}

type repoMem struct {
	mu       sync.Mutex
	jobs     map[string]*jobMem
	assets   map[string]*asset
	mimes    map[string]string
	recursos map[string]string
	sig      int
}

func nuevoRepo() *repoMem {
	return &repoMem{jobs: map[string]*jobMem{}, assets: map[string]*asset{}, mimes: map[string]string{}, recursos: map[string]string{}}
}

func (r *repoMem) TomarJob(_ context.Context, id string, ttl time.Duration) (*job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return nil, errNoExiste
	}
	cp := j.job
	switch {
	case j.Tipo != tipoValidar:
		return &cp, errOtroTipo
	case j.Estado == "done" || j.Estado == "dead_letter":
		return &cp, errTerminado
	case j.Estado == "processing" && time.Since(j.bloqueo) < ttl:
		return &cp, errBloqueado
	}
	j.Estado = "processing"
	j.Intentos++
	j.bloqueo = time.Now()
	cp = j.job
	return &cp, nil
}

func (r *repoMem) RenovarBloqueo(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[id].bloqueo = time.Now()
	return nil
}

func (r *repoMem) BuscarAsset(_ context.Context, id string) (*asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[id]
	if !ok {
		return nil, errors.New("no rows")
	}
	cp := *a
	return &cp, nil
}

func (r *repoMem) EstadoRecurso(_ context.Context, id, estado string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recursos[id] = estado
	return nil
}

func (r *repoMem) GuardarValidacion(_ context.Context, id, checksum, mime string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assets[id].Checksum = &checksum
	r.mimes[id] = mime
	return nil
}

func (r *repoMem) CrearJobSiNoExiste(_ context.Context, assetID, tipo, clave string) (*job, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		if j.clave == clave {
			cp := j.job
			return &cp, false, nil
		}
	}
	r.sig++
	j := &jobMem{job: job{ID: fmt.Sprintf("job-%d", r.sig), AssetID: assetID, Tipo: tipo, Estado: "pending", MaxIntentos: maxIntentos}, clave: clave}
	r.jobs[j.ID] = j
	cp := j.job
	return &cp, true, nil
}

func (r *repoMem) CerrarJob(_ context.Context, j *job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[j.ID].Estado = "done"
	j.Estado = "done"
	return nil
}

func (r *repoMem) FallarJob(_ context.Context, j *job, causa error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	estado := "pending"
	if j.Intentos >= j.MaxIntentos {
		estado = "dead_letter"
	}
	m := r.jobs[j.ID]
	m.Estado, m.Intentos, m.ultError = estado, j.Intentos, causa.Error()
	j.Estado = estado
	return nil
}

func (r *repoMem) contarTipo(tipo string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, j := range r.jobs {
		if j.Tipo == tipo {
			n++
		}
	}
	return n
}

type almacenMem struct {
	mu       sync.Mutex
	objetos  map[string][]byte
	fallos   int // cuántas aperturas fallan antes de funcionar
	lecturas int
	espera   chan struct{}
}

func (a *almacenMem) Abrir(_ context.Context, clave string) (io.ReadCloser, error) {
	a.mu.Lock()
	a.lecturas++
	if a.fallos > 0 {
		a.fallos--
		a.mu.Unlock()
		return nil, errors.New("storage: 503")
	}
	b, ok := a.objetos[clave]
	espera := a.espera
	a.mu.Unlock()
	if espera != nil {
		<-espera
	}
	if !ok {
		return nil, errors.New("storage: 404")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (a *almacenMem) Borrar(_ context.Context, clave string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.objetos, clave)
	return nil
}

type colaMem struct {
	mu         sync.Mutex
	publicados []string
	fallos     int
}

func (c *colaMem) Publicar(_ context.Context, jobID, tipo string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fallos > 0 {
		c.fallos--
		return errors.New("pubsub: 503")
	}
	c.publicados = append(c.publicados, jobID+"/"+tipo)
	return nil
}

// escenario arma un job validate_upload pendiente sobre un asset.
func escenario(kind string, datos []byte, declarado *string) (*validador, *repoMem, *almacenMem, *colaMem) {
	repo := nuevoRepo()
	repo.assets["a1"] = &asset{ID: "a1", ResourceID: "r1", Kind: kind, Clave: "resources/r1/original", Checksum: declarado}
	repo.jobs["v1"] = &jobMem{job: job{ID: "v1", AssetID: "a1", Tipo: tipoValidar, Estado: "pending", MaxIntentos: maxIntentos}, clave: "a1:validate_upload"}
	alm := &almacenMem{objetos: map[string][]byte{"resources/r1/original": datos}}
	c := &colaMem{}
	return &validador{repo: repo, almacen: alm, cola: c, ttl: time.Minute}, repo, alm, c
}

var logPrueba = slog.New(slog.NewTextHandler(io.Discard, nil))

// ---------- pruebas ----------

func TestCargaValidaEncolaEscaneo(t *testing.T) {
	ok := sha(pngMin)
	v, repo, alm, c := escenario("image", pngMin, &ok)

	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
		t.Fatalf("procesar: %v", err)
	}
	if e := repo.jobs["v1"].Estado; e != "done" {
		t.Errorf("job = %s, quiero done", e)
	}
	if repo.mimes["a1"] != "image/png" || *repo.assets["a1"].Checksum != ok {
		t.Errorf("asset no guardó los valores reales: mime=%q", repo.mimes["a1"])
	}
	if repo.recursos["r1"] != "processing" {
		t.Errorf("recurso = %s, quiero processing (el worker lo pone en ready después)", repo.recursos["r1"])
	}
	if n := repo.contarTipo(tipoEscaneo); n != 1 {
		t.Fatalf("scan_antivirus creados = %d, quiero 1", n)
	}
	if len(c.publicados) != 1 || c.publicados[0] != "job-1/scan_antivirus" {
		t.Errorf("publicados = %v", c.publicados)
	}
	if _, sigue := alm.objetos["resources/r1/original"]; !sigue {
		t.Error("una carga válida no debe borrarse")
	}
}

func TestReentregaNoDuplica(t *testing.T) {
	v, repo, alm, c := escenario("pdf", pdfMin, nil)
	for i := 0; i < 3; i++ {
		if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
			t.Fatalf("entrega %d: %v", i+1, err)
		}
	}
	if alm.lecturas != 1 {
		t.Errorf("el original se leyó %d veces, quiero 1", alm.lecturas)
	}
	if n := repo.contarTipo(tipoEscaneo); n != 1 || len(c.publicados) != 1 {
		t.Errorf("scan_antivirus creados=%d publicados=%d, quiero 1 y 1", n, len(c.publicados))
	}
}

func TestEntregasSimultaneas(t *testing.T) {
	v, repo, alm, c := escenario("pdf", pdfMin, nil)
	alm.espera = make(chan struct{})

	errs := make(chan error, 2)
	go func() { errs <- v.procesar(context.Background(), "v1", logPrueba) }()
	// La primera entrega ya tomó el job y está leyendo el original.
	for {
		alm.mu.Lock()
		n := alm.lecturas
		alm.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := v.procesar(context.Background(), "v1", logPrueba); !errors.Is(err, errBloqueado) {
		t.Fatalf("segunda entrega = %v, quiero errBloqueado (Eventarc reintenta)", err)
	}
	close(alm.espera)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if repo.jobs["v1"].Intentos != 1 || len(c.publicados) != 1 {
		t.Errorf("intentos=%d publicados=%d, quiero 1 y 1", repo.jobs["v1"].Intentos, len(c.publicados))
	}
}

func TestRechazoEsPermanente(t *testing.T) {
	malo := "deadbeef"
	v, repo, alm, c := escenario("image", pngMin, &malo)

	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
		t.Fatalf("un rechazo se confirma (nil), salió %v", err)
	}
	j := repo.jobs["v1"]
	if j.Estado != "dead_letter" || j.Intentos != maxIntentos {
		t.Errorf("job = %s con %d intentos, quiero dead_letter con %d", j.Estado, j.Intentos, maxIntentos)
	}
	if repo.recursos["r1"] != "failed" {
		t.Errorf("recurso = %s, quiero failed", repo.recursos["r1"])
	}
	if _, sigue := alm.objetos["resources/r1/original"]; sigue {
		t.Error("el original rechazado debe borrarse")
	}
	if len(c.publicados) != 0 || repo.contarTipo(tipoEscaneo) != 0 {
		t.Error("un rechazo no encola scan_antivirus")
	}
	if *repo.assets["a1"].Checksum != malo {
		t.Error("un rechazo no debe sobrescribir el checksum declarado")
	}
}

func TestPresentacionSinFirmaZip(t *testing.T) {
	v, repo, _, _ := escenario("presentation", pdfMin, nil)
	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
		t.Fatal(err)
	}
	if repo.jobs["v1"].Estado != "dead_letter" {
		t.Errorf("job = %s, quiero dead_letter", repo.jobs["v1"].Estado)
	}
}

func TestErrorTransitorioReintentaHastaDeadLetter(t *testing.T) {
	v, repo, alm, _ := escenario("pdf", pdfMin, nil)
	alm.fallos = 10

	for i := 1; i <= 2; i++ {
		err := v.procesar(context.Background(), "v1", logPrueba)
		if err == nil {
			t.Fatalf("intento %d: un error transitorio debe devolver error", i)
		}
		if j := repo.jobs["v1"]; j.Estado != "pending" || j.Intentos != i {
			t.Fatalf("intento %d: job = %s/%d, quiero pending/%d", i, j.Estado, j.Intentos, i)
		}
	}
	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
		t.Fatalf("tercer intento: sin intentos se confirma, salió %v", err)
	}
	if repo.jobs["v1"].Estado != "dead_letter" || repo.recursos["r1"] != "failed" {
		t.Errorf("job=%s recurso=%s, quiero dead_letter y failed", repo.jobs["v1"].Estado, repo.recursos["r1"])
	}
	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil || alm.lecturas != 3 {
		t.Errorf("una entrega tardía no hace nada: err=%v lecturas=%d", err, alm.lecturas)
	}
}

func TestFallaPublicarYReintentoNoDuplica(t *testing.T) {
	v, repo, _, c := escenario("video", nil, nil)
	repo.assets["a1"].Kind = "file" // cualquier contenido pasa
	c.fallos = 1

	if err := v.procesar(context.Background(), "v1", logPrueba); err == nil {
		t.Fatal("si publicar falla, Eventarc debe reintentar")
	}
	if repo.jobs["v1"].Estado != "pending" {
		t.Fatalf("job = %s, quiero pending", repo.jobs["v1"].Estado)
	}
	if err := v.procesar(context.Background(), "v1", logPrueba); err != nil {
		t.Fatal(err)
	}
	if n := repo.contarTipo(tipoEscaneo); n != 1 || len(c.publicados) != 1 {
		t.Errorf("scan_antivirus creados=%d publicados=%d, quiero 1 y 1", n, len(c.publicados))
	}
	if repo.jobs["v1"].Estado != "done" {
		t.Errorf("job = %s, quiero done", repo.jobs["v1"].Estado)
	}
}

func TestJobsQueNoSonDeLaFuncion(t *testing.T) {
	v, repo, alm, _ := escenario("pdf", pdfMin, nil)
	repo.jobs["s1"] = &jobMem{job: job{ID: "s1", AssetID: "a1", Tipo: tipoEscaneo, Estado: "pending", MaxIntentos: 3}}

	for _, id := range []string{"s1", "no-existe"} {
		if err := v.procesar(context.Background(), id, logPrueba); err != nil {
			t.Errorf("%s: se confirma sin hacer nada, salió %v", id, err)
		}
	}
	if repo.jobs["s1"].Estado != "pending" || repo.jobs["s1"].Intentos != 0 || alm.lecturas != 0 {
		t.Error("la función no debe tocar un job de otro tipo")
	}
}

func TestMensajeMalFormadoSeConfirma(t *testing.T) {
	for _, data := range []string{`{"message":{"data":"bm8gZXMganNvbg=="}}`, `{"message":{"data":"e30="}}`} {
		e := event.New()
		e.SetID("1")
		e.SetSource("//pubsub.googleapis.com/projects/p/topics/t")
		e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
		if err := e.SetData("application/json", []byte(data)); err != nil {
			t.Fatal(err)
		}
		if err := ValidarCarga(context.Background(), e); err != nil {
			t.Errorf("%s: un mensaje mal formado se confirma, salió %v", data, err)
		}
	}
}
