import { cliente } from './cliente'

export type OpcionQuiz = {
  id: string
  text: string
  position: number
}

export type PreguntaQuiz = {
  id: string
  stable_id: string
  text: string
  position: number
  points: number
  options: OpcionQuiz[]
}

export type SnapshotQuiz = {
  quiz_id: string
  questions: PreguntaQuiz[]
}

export type IntentoQuiz = {
  id: string
  quiz_id: string
  status: 'in_progress' | 'submitted' | 'expired'
  score?: number
  percentage?: number
  passed?: boolean
  snapshot: SnapshotQuiz
  started_at: string
  expires_at?: string
  submitted_at?: string
}

export type RespuestaIntento = {
  id: string
  attempt_id: string
  question_id: string
  selected_option_id?: string | null
}

export type DetalleIntento = {
  attempt: IntentoQuiz
  answers: RespuestaIntento[]
}

export function iniciarIntento(quizId: string) {
  return cliente<IntentoQuiz>(`/quizzes/${quizId}/attempts`, {
    method: 'POST',
  })
}

export function obtenerIntento(attemptId: string) {
  return cliente<DetalleIntento>(`/attempts/${attemptId}`)
}

export function guardarRespuesta(
  attemptId: string,
  questionId: string,
  selectedOptionId: string | null,
) {
  return cliente<RespuestaIntento>(
    `/attempts/${attemptId}/answers/${questionId}`,
    {
      method: 'PUT',
      body: {
        selected_option_id: selectedOptionId,
      },
    },
  )
}

export function enviarIntento(
  attemptId: string,
  idempotencyKey: string,
) {
  return cliente<IntentoQuiz>(`/attempts/${attemptId}/submit`, {
    method: 'POST',
    headers: {
      'Idempotency-Key': idempotencyKey,
    },
  })
}