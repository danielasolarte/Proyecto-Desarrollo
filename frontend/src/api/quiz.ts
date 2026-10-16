// Autoría de quizzes. Las respuestas del backend usan los nombres de campo de
// Go (ID, Text, IsCorrect...) porque los modelos del módulo de quizzes no
// tienen etiquetas json; aquí se tipan tal cual llegan.
import { cliente } from './cliente'

export type ModoRetroalimentacion = 'none' | 'after_submit' | 'after_pass'

export interface Quiz {
  ID: string
  ResourceID: string
  PassingScore: number
  MaxAttempts: number | null
  TimeLimitSeconds: number | null
  FeedbackMode: ModoRetroalimentacion
}
export interface Opcion { ID: string; Text: string; Position: number; IsCorrect: boolean }
export interface Pregunta { ID: string; Text: string; Position: number; Points: number; Options: Opcion[] | null }
export interface QuizAutoria { quiz: Quiz; questions: Pregunta[] | null }

export const apiQuiz = {
  delRecurso: (resourceId: string) => cliente<QuizAutoria>(`/resources/${encodeURIComponent(resourceId)}/quiz`),
  crear: (resourceId: string, datos: { passing_score: number; max_attempts: number | null; time_limit_seconds: number | null; feedback_mode: ModoRetroalimentacion }) =>
    cliente<Quiz>(`/resources/${encodeURIComponent(resourceId)}/quiz`, { method: 'POST', body: datos }),
  crearPregunta: (quizId: string, datos: { text: string; position: number; points: number }) =>
    cliente<Pregunta>(`/quizzes/${encodeURIComponent(quizId)}/questions`, { method: 'POST', body: datos }),
  crearOpcion: (preguntaId: string, datos: { text: string; position: number; is_correct: boolean }) =>
    cliente<Opcion>(`/questions/${encodeURIComponent(preguntaId)}/options`, { method: 'POST', body: datos }),
}

export const nombreModo: Record<ModoRetroalimentacion, string> = {
  none: 'No mostrar respuestas correctas',
  after_submit: 'Mostrar al enviar el intento',
  after_pass: 'Mostrar solo cuando apruebe',
}

/** Valida una pregunta nueva antes de enviarla. Devuelve el problema o null. */
export function problemaPregunta(texto: string, puntos: number, opciones: { texto: string }[], correcta: number): string | null {
  if (!texto.trim()) return 'Escribe el enunciado de la pregunta.'
  if (!(puntos > 0)) return 'Los puntos deben ser mayores que cero.'
  const llenas = opciones.filter(o => o.texto.trim())
  if (llenas.length < 2) return 'La pregunta necesita al menos dos opciones con texto.'
  if (correcta < 0 || correcta >= opciones.length || !opciones[correcta].texto.trim()) return 'Marca cuál opción es la correcta.'
  return null
}
