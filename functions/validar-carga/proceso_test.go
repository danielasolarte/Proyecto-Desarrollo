package validarcarga

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// almacenFalso simula media_jobs, media_assets y resources en memoria, con
// las mismas reglas que las consultas de postgres.go.
type almacenFalso struct {
	mu        sync.Mutex
	trabajos  map[string]*trabajo
	claves    map[string]string // idempotency_key -> id
	activos   map[string]*activo
	recursos  map[string]string
	errores   map[string]string // last_error
	guardados map[string][2]string
	tomado    map[string]bool
	fallaEn   string // nombre del método que debe fallar
	renovados int
	nuevoID   int
}

func nuevoAlmacen() *almacenFalso {
	return &almacenFalso{
		trabajos: map[string]*trabajo{}, claves: map[string]string{}, activos: map[string]*activo{},
		recursos: map[string]string{}, errores: map[string]string{}, guardados: map[string][2]string{},
		tomado: map[string]bool{},
	}
}

var errBD = errors.New("postgres no responde")

func (s *almacenFalso) falla(m string) error {
	if s.fallaEn == m {
		return errBD
	}
	return nil
}

func (s *almacenFalso) TomarTrabajo(_ context.Context, id string, _ time.Duration) (*trabajo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.falla("TomarTrabajo"); err != nil {
		return nil, err
	}
	t, ok := s.trabajos[id]
	if !ok {
		return nil, errNoExiste
	}
	if t.Estado == estadoTerminado || t.Estado == estadoDeadLetter {
		c := *t
		return &c, errTrabajoTerminado
	}
	if s.tomado[id] {
		c := *t
		return &c, errTrabajoTomado
	}
	t.Estado, t.Intentos = estadoEnProceso, t.Intentos+1
	s.tomado[id] = true
	c := *t
	return &c, nil
}

func (s *almacenFalso) RenovarBloqueo(context.Context, string) error {
	s.mu.Lock()
	s.renovados++
	s.mu.Unlock()
	return nil
}

func (s *almacenFalso) Activo(_ context.Context, id string) (*activo, error) {
	if err := s.falla("Activo"); err != nil {
		return nil, err
	}
	a, ok := s.activos[id]
	if !ok {
		return nil, errNoExiste
	}
	return a, nil
}

func (s *almacenFalso) GuardarValidacion(_ context.Context, id, checksum, mime string) error {
	if err := s.falla("GuardarValidacion"); err != nil {
		return err
	}
	s.guardados[id] = [2]string{checksum, mime}
	return nil
}

func (s *almacenFalso) EstadoRecurso(_ context.Context, id, estado string) error {
	s.recursos[id] = estado
	return nil
}

func (s *almacenFalso) CerrarTrabajo(_ context.Context, t *trabajo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trabajos[t.ID].Estado = estadoTerminado
	s.tomado[t.ID] = false
	return nil
}

func (s *almacenFalso) FallarTrabajo(_ context.Context, t *trabajo, causa string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.trabajos[t.ID]
	g.Intentos = t.Intentos
	g.Estado = estadoPendiente
	if t.Intentos >= t.MaxIntentos {
		g.Estado = estadoDeadLetter
	}
	t.Estado = g.Estado
	s.errores[t.ID] = causa
	s.tomado[t.ID] = false
	return nil
}

func (s *almacenFalso) TrabajoSiguiente(_ context.Context, assetID, tipo, clave string) (*trabajo, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.falla("TrabajoSiguiente"); err != nil {
		return nil, false, err
	}
	if id, ok := s.claves[clave]; ok {
		t := s.trabajos[id]
		return t, t.Estado == estadoPendiente, nil
	}
	s.nuevoID++
	t := &trabajo{ID: "sig-" + string(rune('0'+s.nuevoID)), AssetID: assetID, Tipo: tipo, Estado: estadoPendiente, MaxIntentos: 3}
	s.trabajos[t.ID], s.claves[clave] = t, t.ID
	return t, true, nil
}

type bucketFalso struct {
	objetos  map[string][]byte
	borrados []string
	falla    bool
}

func (b *bucketFalso) Abrir(_ context.Context, clave string) (io.ReadCloser, error) {
	if b.falla {
		return nil, errors.New("storage no responde")
	}
	o, ok := b.objetos[clave]
	if !ok {
		return nil, errors.New("no existe " + clave)
	}
	return io.NopCloser(bytes.NewReader(o)), nil
}

func (b *bucketFalso) Borrar(_ context.Context, clave string) error {
	b.borrados = append(b.borrados, clave)
	delete(b.objetos, clave)
	return nil
}

type colaFalsa struct {
	publicados []string
	falla      bool
}

func (q *colaFalsa) Publicar(_ context.Context, id, tipo string) error {
	if q.falla {
		return errors.New("pubsub no responde")
	}
	q.publicados = append(q.publicados, id+"|"+tipo)
	return nil
}

func escenario(kind string, contenido []byte, declarado *string) (*validador, *almacenFalso, *bucketFalso, *colaFalsa) {
	db := nuevoAlmacen()
	db.trabajos["j1"] = &trabajo{ID: "j1", AssetID: "a1", Tipo: tipoValidar, Estado: estadoPendiente, MaxIntentos: 3}
	db.claves["a1:validate_upload"] = "j1"
	db.activos["a1"] = &activo{ID: "a1", RecursoID: "r1", Kind: kind, ClaveBlob: "resources/r1/original", Checksum: declarado}
	b := &bucketFalso{objetos: map[string][]byte{"resources/r1/original": contenido}}
	q := &colaFalsa{}
	v := &validador{db: db, blobs: b, cola: q, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return v, db, b, q
}

func TestArchivoValidoGuardaYEncolaAntivirus(t *testing.T) {
	bien := sha(pdfFalso)
	v, db, b, q := escenario("pdf", pdfFalso, &bien)
	if err := v.procesar(context.Background(), "j1"); err != nil {
		t.Fatalf("debía confirmar: %v", err)
	}
	if db.trabajos["j1"].Estado != estadoTerminado {
		t.Fatalf("estado %s", db.trabajos["j1"].Estado)
	}
	if g := db.guardados["a1"]; g[0] != bien || g[1] != "application/pdf" {
		t.Fatalf("guardado %v", g)
	}
	if len(q.publicados) != 1 || !strings.HasSuffix(q.publicados[0], "|scan_antivirus") {
		t.Fatalf("publicados %v", q.publicados)
	}
	if db.recursos["r1"] != recursoEnProceso || len(b.borrados) != 0 {
		t.Fatalf("recurso %s, borrados %v", db.recursos["r1"], b.borrados)
	}
}

func TestMensajeRepetidoNoDuplica(t *testing.T) {
	v, db, _, q := escenario("pdf", pdfFalso, nil)
	ctx := context.Background()
	if err := v.procesar(ctx, "j1"); err != nil {
		t.Fatal(err)
	}
	if err := v.procesar(ctx, "j1"); err != nil {
		t.Fatalf("el repetido debía confirmarse: %v", err)
	}
	if len(q.publicados) != 1 || db.nuevoID != 1 {
		t.Fatalf("se duplicó el antivirus: publicados %v, creados %d", q.publicados, db.nuevoID)
	}
}

func TestChecksumFalsoBorraYQuedaDeadLetter(t *testing.T) {
	mal := strings.Repeat("0", 64)
	v, db, b, q := escenario("pdf", pdfFalso, &mal)
	if err := v.procesar(context.Background(), "j1"); err != nil {
		t.Fatalf("un rechazo se confirma sin reintento: %v", err)
	}
	if db.trabajos["j1"].Estado != estadoDeadLetter || !strings.Contains(db.errores["j1"], "checksum") {
		t.Fatalf("estado %s, error %q", db.trabajos["j1"].Estado, db.errores["j1"])
	}
	if len(b.borrados) != 1 || db.recursos["r1"] != recursoFallido || len(q.publicados) != 0 {
		t.Fatalf("borrados %v, recurso %s, publicados %v", b.borrados, db.recursos["r1"], q.publicados)
	}
}

func TestPresentacionSinZipSeRechaza(t *testing.T) {
	v, db, b, _ := escenario("presentation", pdfFalso, nil)
	if err := v.procesar(context.Background(), "j1"); err != nil {
		t.Fatal(err)
	}
	if db.trabajos["j1"].Estado != estadoDeadLetter || len(b.borrados) != 1 {
		t.Fatalf("estado %s, borrados %v", db.trabajos["j1"].Estado, b.borrados)
	}
}

func TestErrorTransitorioReintentaYAlTerceroQuedaDeadLetter(t *testing.T) {
	v, db, b, q := escenario("pdf", pdfFalso, nil)
	b.falla = true
	ctx := context.Background()
	for i := 1; i <= 2; i++ {
		if err := v.procesar(ctx, "j1"); err == nil {
			t.Fatalf("intento %d: debía pedir reintento", i)
		}
		if db.trabajos["j1"].Estado != estadoPendiente {
			t.Fatalf("intento %d: estado %s", i, db.trabajos["j1"].Estado)
		}
	}
	if err := v.procesar(ctx, "j1"); err != nil {
		t.Fatalf("al tercer intento debía confirmar: %v", err)
	}
	if db.trabajos["j1"].Estado != estadoDeadLetter || db.recursos["r1"] != recursoFallido || len(q.publicados) != 0 {
		t.Fatalf("estado %s, recurso %s", db.trabajos["j1"].Estado, db.recursos["r1"])
	}
}

func TestFallaPubSubYElReintentoNoDuplica(t *testing.T) {
	v, db, _, q := escenario("pdf", pdfFalso, nil)
	q.falla = true
	ctx := context.Background()
	if err := v.procesar(ctx, "j1"); err == nil {
		t.Fatal("debía pedir reintento")
	}
	q.falla = false
	if err := v.procesar(ctx, "j1"); err != nil {
		t.Fatal(err)
	}
	if db.nuevoID != 1 || len(q.publicados) != 1 || db.trabajos["j1"].Estado != estadoTerminado {
		t.Fatalf("creados %d, publicados %v, estado %s", db.nuevoID, q.publicados, db.trabajos["j1"].Estado)
	}
}

func TestTrabajoTomadoPorOtraInstanciaPideReintento(t *testing.T) {
	v, db, _, _ := escenario("pdf", pdfFalso, nil)
	db.tomado["j1"] = true
	if err := v.procesar(context.Background(), "j1"); !errors.Is(err, errTrabajoTomado) {
		t.Fatalf("esperaba errTrabajoTomado, obtuve %v", err)
	}
}

func TestTrabajoInexistenteOdeOtroTipoSeConfirman(t *testing.T) {
	v, db, _, _ := escenario("pdf", pdfFalso, nil)
	if err := v.procesar(context.Background(), "no-existe"); err != nil {
		t.Fatalf("inexistente: %v", err)
	}
	db.trabajos["j2"] = &trabajo{ID: "j2", AssetID: "a1", Tipo: tipoAntivirus, Estado: estadoPendiente, MaxIntentos: 3}
	if err := v.procesar(context.Background(), "j2"); err != nil {
		t.Fatalf("otro tipo: %v", err)
	}
	if db.trabajos["j2"].Estado != estadoDeadLetter {
		t.Fatalf("otro tipo debía quedar dead_letter, quedó %s", db.trabajos["j2"].Estado)
	}
}

func TestErrorDeBaseAlTomarPideReintento(t *testing.T) {
	v, db, _, _ := escenario("pdf", pdfFalso, nil)
	db.fallaEn = "TomarTrabajo"
	if err := v.procesar(context.Background(), "j1"); err == nil {
		t.Fatal("debía pedir reintento")
	}
}

func TestLatidoRenuevaElBloqueo(t *testing.T) {
	v, db, _, _ := escenario("pdf", pdfFalso, nil)
	v.latido = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	go v.mantenerBloqueo(ctx, "j1")
	time.Sleep(40 * time.Millisecond)
	cancel()
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.renovados < 2 {
		t.Fatalf("renovó %d veces", db.renovados)
	}
}
