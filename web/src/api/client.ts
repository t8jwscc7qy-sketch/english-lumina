import type { components } from './schema.gen'

type ServerErrorCode = components['schemas']['Error']['error']['code']

/** Коды сервера плюс «network» — запрос вообще не дошёл до сервера. */
export type ErrorCode = ServerErrorCode | 'network'

/** Ошибка API: статус 0 означает потерю связи. Текст на русском, его можно показывать пользователю. */
export class ApiError extends Error {
  readonly status: number
  readonly code: ErrorCode

  constructor(status: number, code: ErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const NETWORK_MESSAGE = 'Нет связи с сервером. Проверь, что он запущен, и попробуй ещё раз.'
const INTERNAL_MESSAGE = 'Сервер ответил неожиданно. Попробуй ещё раз чуть позже.'

async function parseError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as Partial<components['schemas']['Error']>
    const e = body.error
    if (e && typeof e.code === 'string' && typeof e.message === 'string') {
      return new ApiError(res.status, e.code, e.message)
    }
  } catch {
    // тело не JSON — например, страница прокси; ниже вернём общую ошибку
  }
  return new ApiError(res.status, 'internal', INTERNAL_MESSAGE)
}

async function request(path: string, init: RequestInit = {}): Promise<Response> {
  let res: Response
  try {
    res = await fetch(path, { credentials: 'same-origin', ...init })
  } catch {
    throw new ApiError(0, 'network', NETWORK_MESSAGE)
  }
  if (!res.ok) throw await parseError(res)
  return res
}

export async function login(login: string, password: string): Promise<void> {
  await request('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login, password }),
  })
}

export async function logout(): Promise<void> {
  await request('/api/v1/auth/logout', { method: 'POST' })
}

export async function getMe(): Promise<components['schemas']['Me']> {
  const res = await request('/api/v1/auth/me')
  return (await res.json()) as components['schemas']['Me']
}

export type Settings = components['schemas']['Settings']

export async function getSettings(): Promise<Settings> {
  const res = await request('/api/v1/settings')
  return (await res.json()) as Settings
}

export async function putSettings(settings: Settings): Promise<Settings> {
  const res = await request('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  return (await res.json()) as Settings
}

export type SourceList = components['schemas']['SourceList']

export async function getSources(): Promise<SourceList> {
  const res = await request('/api/v1/sources')
  return (await res.json()) as SourceList
}

export type WordList = components['schemas']['WordList']

/** Поиск слова по началу; q экранируется, чтобы & / % и пробелы не ломали адрес. */
export async function searchWords(q: string): Promise<WordList> {
  const res = await request(`/api/v1/words?q=${encodeURIComponent(q)}`)
  return (await res.json()) as WordList
}

export type ExampleList = components['schemas']['ExampleList']

/** Примеры предложений для слова; lemma и pos экранируются (в лемме бывает косая черта). */
export async function getWordExamples(lemma: string, pos: string): Promise<ExampleList> {
  const res = await request(`/api/v1/words/examples?lemma=${encodeURIComponent(lemma)}&pos=${encodeURIComponent(pos)}`)
  return (await res.json()) as ExampleList
}

export type CardQueue = components['schemas']['CardQueue']
export type QueueCard = components['schemas']['QueueCard']

export async function getCardQueue(): Promise<CardQueue> {
  const res = await request('/api/v1/cards/queue')
  return (await res.json()) as CardQueue
}

/** Оценка карточки (1 забыл … 4 легко). key — ключ идемпотентности: повтор с тем же ключом не применяется дважды. */
export async function reviewCard(cardId: number, rating: 1 | 2 | 3 | 4, key: string): Promise<CardQueue> {
  const res = await request('/api/v1/cards/review', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ cardId, rating, key }),
  })
  return (await res.json()) as CardQueue
}

export async function addCard(lemma: string, pos: string): Promise<{ cardId: number; created: boolean }> {
  const res = await request('/api/v1/cards', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ lemma, pos }),
  })
  return (await res.json()) as { cardId: number; created: boolean }
}

export type Exercise = components['schemas']['Exercise']
export type AnswerResult = components['schemas']['AnswerResult']
export type Rating = components['schemas']['Rating']
export type TopicList = components['schemas']['TopicList']

/** Следующее упражнение; 404 с кодом no_exercise значит, что подходящих пока нет. */
export async function getNextExercise(): Promise<Exercise> {
  const res = await request('/api/v1/exercises/next')
  return (await res.json()) as Exercise
}

/** Ответ на упражнение; key — ключ идемпотентности, один на попытку. */
export async function answerExercise(exerciseId: number, given: string, key: string): Promise<AnswerResult> {
  const res = await request('/api/v1/exercises/answer', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ exerciseId, given, key }),
  })
  return (await res.json()) as AnswerResult
}

export async function listExerciseTopics(): Promise<TopicList> {
  const res = await request('/api/v1/exercises/topics')
  return (await res.json()) as TopicList
}

export type MistakeList = components['schemas']['MistakeList']
export type Mistake = components['schemas']['Mistake']

/** Банк ошибок: открытые ошибки по сроку повтора и три счётчика. */
export async function getMistakes(): Promise<MistakeList> {
  const res = await request('/api/v1/mistakes')
  return (await res.json()) as MistakeList
}

export type ProgressToday = components['schemas']['ProgressToday']

/** Минуты, серия и счётчики дня из журнала событий. */
export async function getProgressToday(): Promise<ProgressToday> {
  const res = await request('/api/v1/progress/today')
  return (await res.json()) as ProgressToday
}

export type ProgressOverview = components['schemas']['ProgressOverview']

/** Факты для страницы «Прогресс»: старт, темп, накопленные часы по суткам. */
export async function getProgressOverview(): Promise<ProgressOverview> {
  const res = await request('/api/v1/progress/overview')
  return (await res.json()) as ProgressOverview
}

export type MilestoneList = components['schemas']['MilestoneList']
export type Milestone = components['schemas']['Milestone']

/** Вехи занятий и те, что достигнуты сегодня. */
export async function getMilestones(): Promise<MilestoneList> {
  const res = await request('/api/v1/milestones')
  return (await res.json()) as MilestoneList
}
