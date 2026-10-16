// Puntos de integración del editor del curso con los componentes de Mateo
// (multimedia) y Camila (quiz). Cuando existan sus componentes, se cambia
// solo este archivo:
//
//   Mateo:  import { CargaArchivo } from '../../componentes/media/CargaArchivo'
//           y en RanuraMultimedia: return <CargaArchivo resourceId={recurso.id} alCambiar={alCambiar} />
//   Camila: import { EditorQuiz } from '../../componentes/quiz/EditorQuiz'
//           y en RanuraQuiz: return <EditorQuiz resourceId={recurso.id} />
//
// Contrato acordado: el componente recibe el id del recurso y, si cambia el
// estado del recurso (por ejemplo, termina una carga), llama a alCambiar()
// para que el editor recargue el árbol y los problemas de publicación.
import type { EstadoProcesamiento, Recurso } from '../../api/cursos'
import { Procesando } from '../../componentes/estados/Estados'

const textoEstado: Record<EstadoProcesamiento, string> = {
  pending: 'Sin archivo cargado',
  processing: 'Procesando',
  ready: 'Disponible',
  failed: 'Falló el procesamiento',
}

export function EstadoMultimedia({ estado }: { estado?: EstadoProcesamiento }) {
  if (!estado) return null
  if (estado === 'processing') return <Procesando mensaje="El archivo se está procesando. Aún no está disponible." />
  return <span className={`estado-proceso proceso-${estado}`}>{textoEstado[estado]}</span>
}

export function RanuraMultimedia({ recurso }: { recurso: Recurso; alCambiar: () => void }) {
  return <div className="ranura">
    <EstadoMultimedia estado={recurso.processing_status} />
    <p className="ranura-nota">La carga del archivo se integra con el componente de multimedia.</p>
  </div>
}

export function RanuraQuiz({ recurso }: { recurso: Recurso }) {
  return <div className="ranura"><p className="ranura-nota">Las preguntas del quiz «{recurso.title}» se editan con el componente de quiz.</p></div>
}
