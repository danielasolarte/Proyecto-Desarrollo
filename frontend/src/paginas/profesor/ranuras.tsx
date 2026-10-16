// Puntos de integración del editor del curso con los componentes de
// multimedia (CargaArchivo) y de quiz (EditorQuiz).
//
// Contrato: el componente recibe el id del recurso y, si cambia el estado del
// recurso (por ejemplo, termina una carga), llama a alCambiar() para que el
// editor recargue el árbol y los problemas de publicación.
import type { EstadoProcesamiento, Recurso } from '../../api/cursos'
import { Procesando } from '../../componentes/estados/Estados'
import { CargaArchivo } from '../../componentes/media/CargaArchivo'
import { EditorQuiz } from '../../componentes/quiz/EditorQuiz'

const textoEstado: Record<EstadoProcesamiento, string> = {
  pending: 'Sin archivo listo',
  processing: 'Procesando',
  ready: 'Disponible',
  failed: 'Falló el procesamiento',
}

export function EstadoMultimedia({ estado }: { estado?: EstadoProcesamiento }) {
  if (!estado) return null
  if (estado === 'processing') return <Procesando mensaje="El archivo se está procesando. Aún no está disponible." />
  return <span className={`estado-proceso proceso-${estado}`}>{textoEstado[estado]}</span>
}

export function RanuraMultimedia({ recurso, editable = true, alCambiar }: { recurso: Recurso; editable?: boolean; alCambiar: () => void }) {
  return <div className="ranura">
    <EstadoMultimedia estado={recurso.processing_status} />
    <CargaArchivo resourceId={recurso.id} tipo={recurso.type} estado={recurso.processing_status} editable={editable} alCambiar={alCambiar} />
  </div>
}

export function RanuraQuiz({ recurso, editable = true }: { recurso: Recurso; editable?: boolean }) {
  return <div className="ranura"><EditorQuiz resourceId={recurso.id} editable={editable} /></div>
}
