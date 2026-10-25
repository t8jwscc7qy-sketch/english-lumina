import type { Page } from '@playwright/test'

export const ME = { login: 'firimon' }

export type Settings = {
  currentLevel: string
  targetLevel: string
  targetDate?: string | null
  dailyMinutes: number
  timezone: string
}

export type Source = {
  name: string
  version: string
  license: string
  url: string
  attribution: string
  rowCount: number
  importedAt: string
}

export const SOURCES: Source[] = [
  {
    name: 'cefrj',
    version: '1.5',
    license: 'Бесплатно для исследований и коммерции с указанием источника (Tono Laboratory, TUFS); при коммерческом использовании условия согласуются отдельно',
    url: 'https://raw.githubusercontent.com/openlanguageprofiles/olp-en-cefrj/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/cefrj-vocabulary-profile-1.5.csv',
    attribution: 'The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies.',
    rowCount: 7799,
    importedAt: '2026-10-23T12:00:00Z',
  },
]

type WordFixture = {
  lemma: string
  pos: string
  level: string
  levels: { source: string; level: string; notes: string[] }[]
  frequencyRank?: number
}

export const WORDS: WordFixture[] = [
  { lemma: 'run', pos: 'verb', level: 'A1', levels: [{ source: 'cefrj', level: 'A1', notes: [] }], frequencyRank: 244 },
  { lemma: 'runner', pos: 'noun', level: 'B1', levels: [{ source: 'cefrj', level: 'B1', notes: [] }], frequencyRank: 5216 },
  { lemma: 'runway', pos: 'noun', level: 'B2', levels: [{ source: 'cefrj', level: 'B2', notes: [] }] },
  {
    lemma: 'complexity',
    pos: 'noun',
    level: 'B2',
    levels: [
      { source: 'cefrj', level: 'B2', notes: [] },
      { source: 'octanove', level: 'C1', notes: ['сложность', 'запутанность'] },
    ],
  },
  { lemma: 'cafe/café', pos: 'noun', level: 'A1', levels: [{ source: 'cefrj', level: 'A1', notes: [] }] },
  // Слово без примеров в Tatoeba: добавить в карточки нельзя (409 no_examples).
  { lemma: 'zebra', pos: 'noun', level: 'A2', levels: [{ source: 'cefrj', level: 'A2', notes: [] }] },
]

/** Предложения для карточек: пропуск (gap) в символах. */
const CARD_SENTENCES: Record<string, unknown> = {
  run: {
    en: { text: 'I like to run every morning before work.', author: 'CK', tatoebaId: 1234567 },
    ru: { text: 'Я люблю бегать по утрам перед работой.', author: 'marafon', tatoebaId: 7654321 },
    gap: { start: 10, end: 13 },
  },
  runner: {
    en: { text: 'My brother is a good runner.', author: 'CM', tatoebaId: 2345678 },
    ru: { text: 'Мой брат хороший бегун.', author: 'ksenija', tatoebaId: 8765432 },
    gap: { start: 21, end: 27 },
  },
  runway: {
    en: { text: 'The plane landed on the runway.', author: 'Hybrid', tatoebaId: 3456789 },
    ru: { text: 'Самолёт приземлился на взлётную полосу.', author: 'a', tatoebaId: 9876543 },
    gap: { start: 24, end: 30 },
  },
}

type MockCard = { id: number; lemma: string; pos: string; level: string; due: boolean }
export type CardReview = { cardId: number; rating: number; key: string }

export const EXAMPLES = {
  examples: [
    {
      en: { text: 'I like to run every morning before work.', author: 'CK', tatoebaId: 1234567 },
      ru: { text: 'Я люблю бегать по утрам перед работой.', author: 'marafon', tatoebaId: 7654321 },
      highlight: { start: 10, end: 13 },
    },
    {
      en: { text: 'They were running along the river.', author: 'Hybrid', tatoebaId: 2345678 },
      ru: { text: 'Они бежали вдоль реки.', author: 'ksenija', tatoebaId: 8765432 },
      highlight: { start: 10, end: 17 },
    },
  ],
}

export const SETTINGS: Settings = {
  currentLevel: 'B1',
  targetLevel: 'B2',
  dailyMinutes: 15,
  timezone: 'Europe/Moscow',
}

export type ExerciseFixture = {
  id: number
  kind: 'gap' | 'order' | 'form' | 'match'
  prompt: string
  translation: string
  choices: string[]
  topic: string
  /** Верный ответ живёт только в подмене сервера, клиенту не отдаётся до ответа. */
  answer: string
  /** Только для order: слова в перемешанном порядке; допустимые порядки (без точки) в `accept`. */
  tiles?: string[]
  accept?: string[]
  /** Только для form: начальная форма в скобках. */
  hint?: string
  /** Только для form: слово, которое ответ возвращает после проверки. */
  word?: { lemma: string; pos: string; inCards: boolean }
  /** Только для form: POST /cards для этого слова отвечает 409 no_examples. */
  noExamples?: boolean
  /** Повтор ошибки: ответ двигает ошибку этого упражнения (ошибка создаётся, если её ещё нет). */
  repeat?: boolean
  /** Только для match: слова, переводы вперемешку и верные пары (до ответа клиенту не отдаются). */
  words?: string[]
  options?: string[]
  pairs?: { word: string; ru: string }[]
}
export type MistakeFixture = {
  id: number; exerciseId?: number; topic: string; kind: string; prompt: string
  given: string; answer: string; streak: number
  /** Через сколько дней вернётся (отрицательное — уже ждёт). */
  dueInDays: number
}
type MockMistake = Omit<MistakeFixture, 'dueInDays'> & { nextDue: string; firstWrongAt: string }
const DAY_MS = 24 * 3600_000
export type ExerciseOpts = {
  queue?: ExerciseFixture[]
  /** Сервер на GET /exercises/next отвечает 404 no_exercise. */
  none?: boolean
  /** POST /exercises/answer отвечает 500, пока флаг включён; тест может его снять. */
  failAnswers?: boolean
}
export type CardAdd = { lemma: string; pos: string }
export type ExerciseAnswer = { exerciseId: number; given: string; key: string }
type TopicRating = { topic: string; rating: number; deviation: number; attempts: number }

export const ORDER_EXERCISE: ExerciseFixture = {
  id: 31, kind: 'order', prompt: 'Вчера я пошёл домой.', translation: 'Вчера я пошёл домой.',
  choices: [], tiles: ['home', 'went', 'yesterday', 'I'], topic: 'skill:order',
  answer: 'Yesterday I went home.', accept: ['Yesterday I went home.', 'I went home yesterday.'],
}

export const MATCH_EXERCISE: ExerciseFixture = {
  id: 51, kind: 'match', prompt: 'Соедини слова с переводами', translation: '', choices: [], topic: 'skill:match',
  answer: 'apple=яблоко;house=дом;water=вода;book=книга;dog=собака',
  words: ['apple', 'house', 'water', 'book', 'dog'],
  options: ['книга', 'яблоко', 'собака', 'вода', 'дом'],
  pairs: [{ word: 'apple', ru: 'яблоко' }, { word: 'house', ru: 'дом' }, { word: 'water', ru: 'вода' }, { word: 'book', ru: 'книга' }, { word: 'dog', ru: 'собака' }],
}

export const FORM_EXERCISE: ExerciseFixture = {
  id: 41, kind: 'form', prompt: 'Yesterday she _____ to the cinema.', hint: 'go',
  translation: 'Вчера она ходила в кино.', choices: ['went', 'goes', 'going', 'gone'],
  topic: 'skill:form', answer: 'went', word: { lemma: 'go', pos: 'verb', inCards: false },
}

export const EXERCISES: ExerciseFixture[] = [
  {
    id: 11, kind: 'gap', prompt: 'Yesterday she _____ to the cinema with her sister.',
    translation: 'Вчера она ходила в кино со своей сестрой.',
    choices: ['went', 'goes', 'going', 'gone'], topic: 'grammar:past_simple', answer: 'went',
  },
  {
    id: 12, kind: 'gap', prompt: 'He is _____ honest man.',
    translation: 'Он честный человек.',
    choices: ['an', 'a', 'the', 'some'], topic: 'grammar:articles', answer: 'an',
  },
  {
    id: 13, kind: 'gap', prompt: 'We arrived _____ the station late.',
    translation: 'Мы приехали на станцию поздно.',
    choices: [], topic: 'grammar:prepositions', answer: 'at',
  },
]

/**
 * Подменяет API Go-сервера. `settings: null` — онбординг не пройден (GET /settings даёт 404);
 * PUT сохраняет присланное и дальше GET его возвращает. `puts` копит тела принятых PUT.
 */
export type LevelSuggestionOpts = {
  /** Предложение на экране; без него отвечает пустым объектом. */
  suggestion?: { direction: 'up' | 'down'; from: string; to: string; correct: number; total: number }
  /** POST /level/suggestion/dismiss отвечает 500. */
  dismissFails?: boolean
  /** Сюда складываются вызовы отказа. */
  dismissed?: { count: number }
}

export type MilestonesOpts = {
  /** id достигнутых вех (streak-3, hours-1, first-card …). */
  reached?: string[]
  /** id вех, достигнутых сегодня (они считаются достигнутыми). */
  newToday?: string[]
  /** GET /milestones отвечает 500. */
  fail?: boolean
}

export type OverviewOpts = {
  /** Сколько последних суток подряд были занятия (0 — занятий не было). */
  activeDays?: number
  /** Минут занятий в каждые из этих суток. */
  minutesPerDay?: number
  /** GET /progress/overview отвечает 500, пока флаг включён. */
  fail?: boolean
}

export type ProgressOpts = {
  /** Минут занятий за сегодня к началу теста. */
  minutes?: number
  /** Сколько минут добавляет каждый ответ на карточку или упражнение (по умолчанию 0,5). */
  step?: number
  streakDays?: number
  freezeUsedOn?: string
  /** GET /progress/today отвечает 500, пока флаг включён. */
  fail?: boolean
}

export async function mockApi(
  page: Page,
  opts: { loggedIn: boolean; settings?: Settings | null; settingsStatus?: number; sources?: Source[]; dueCards?: string[]; exercises?: ExerciseOpts; mistakes?: MistakeFixture[]; closedThisWeek?: number; progress?: ProgressOpts; overview?: OverviewOpts; milestones?: MilestonesOpts; level?: LevelSuggestionOpts },
) {
  let loggedIn = opts.loggedIn
  let settings: Settings | null = opts.settings === undefined ? SETTINGS : opts.settings
  const puts: Settings[] = []

  await page.route('**/api/v1/auth/**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/me')) {
      return loggedIn
        ? route.fulfill({ json: ME })
        : route.fulfill({
            status: 401,
            json: { error: { code: 'unauthorized', message: 'Нужно войти' } },
          })
    }
    if (url.pathname.endsWith('/login')) {
      const body = route.request().postDataJSON() as { password: string }
      if (body.password === 'right-password-1') {
        loggedIn = true
        return route.fulfill({ status: 204 })
      }
      return route.fulfill({
        status: 401,
        json: { error: { code: 'invalid_credentials', message: 'Неверный логин или пароль' } },
      })
    }
    if (url.pathname.endsWith('/logout')) {
      loggedIn = false
      return route.fulfill({ status: 204 })
    }
    return route.fulfill({ status: 404 })
  })

  await page.route('**/api/v1/settings', async (route) => {
    if (opts.settingsStatus) {
      return route.fulfill({
        status: opts.settingsStatus,
        json: { error: { code: 'internal', message: 'Внутренняя ошибка, попробуй ещё раз' } },
      })
    }
    if (route.request().method() === 'PUT') {
      settings = route.request().postDataJSON() as Settings
      puts.push(settings)
      return route.fulfill({ json: settings })
    }
    return settings
      ? route.fulfill({ json: settings })
      : route.fulfill({
          status: 404,
          json: { error: { code: 'onboarding_required', message: 'Сначала настрой цель и уровень' } },
        })
  })

  await page.route('**/api/v1/sources', (route) =>
    route.fulfill({ json: { sources: opts.sources ?? SOURCES } }),
  )

  // Поиск как у сервера: по началу слова без учёта регистра.
  // Карточки в памяти: слова из opts.dueCards уже добавлены и просрочены (лежат в очереди).
  const cards: MockCard[] = (opts.dueCards ?? []).map((lemma, i) => {
    const w = WORDS.find((x) => x.lemma === lemma)
    return { id: i + 1, lemma, pos: w?.pos ?? 'verb', level: w?.level ?? 'A1', due: true }
  })
  const reviews: CardReview[] = []
  const prog = { minutes: opts.progress?.minutes ?? 0, step: opts.progress?.step ?? 0.5, answers: 0, correct: 0, cardsReviewed: 0, opened: 0, closed: 0 }
  const seenKeys = new Set<string>()
  let nextCardId = 100
  const queueCard = (c: MockCard) => ({
    cardId: c.id, lemma: c.lemma, pos: c.pos, level: c.level, state: 0,
    intervals: { again: 60, hard: 360, good: 600, easy: 345_600 },
    ...(CARD_SENTENCES[c.lemma] ? { sentence: CARD_SENTENCES[c.lemma] } : {}),
  })
  const queueBody = () => {
    const due = cards.filter((c) => c.due)
    return { dueCount: due.length, totalCount: cards.length, ...(due[0] ? { card: queueCard(due[0]) } : {}) }
  }
  // Слова из упражнений «выбор формы» (не из WORDS): значение true — отвечать 409 no_examples.
  const formWords = new Map<string, boolean>(
    (opts.exercises?.queue ?? []).filter((e) => e.word).map((e) => [e.word!.lemma, !!e.noExamples]),
  )
  const apiError = (status: number, code: string, message: string) => ({ status, json: { error: { code, message } } })

  await page.route('**/api/v1/words**', (route) => {
    const q = (new URL(route.request().url()).searchParams.get('q') ?? '').trim().toLowerCase()
    const words = q
      ? WORDS.filter((w) => w.lemma.toLowerCase().startsWith(q)).map((w) => ({ ...w, inCards: cards.some((c) => c.lemma === w.lemma) }))
      : []
    return route.fulfill({ json: { words } })
  })

  // Примеры для слова. Регистрируется после поиска слов: Playwright сначала пробует поздние маршруты.
  await page.route('**/api/v1/words/examples**', (route) => route.fulfill({ json: EXAMPLES }))

  const cardAdds: CardAdd[] = []
  await page.route('**/api/v1/cards**', async (route) => {
    const url = new URL(route.request().url())
    const method = route.request().method()
    if (method === 'POST' && url.pathname.endsWith('/cards') && formWords.has(route.request().postDataJSON().lemma)) {
      const body = route.request().postDataJSON() as CardAdd
      cardAdds.push(body)
      if (formWords.get(body.lemma)) return route.fulfill(apiError(409, 'no_examples', 'Для этого слова нет подходящих примеров, карточку не собрать'))
      return route.fulfill({ status: 201, json: { cardId: nextCardId++, created: true } })
    }
    if (url.pathname.endsWith('/cards/queue')) return route.fulfill({ json: queueBody() })
    if (url.pathname.endsWith('/cards/review')) {
      const body = route.request().postDataJSON() as CardReview
      reviews.push(body)
      const card = cards.find((c) => c.id === body.cardId)
      if (!card) return route.fulfill(apiError(404, 'not_found', 'Такой карточки нет'))
      if (!seenKeys.has(body.key)) {
        seenKeys.add(body.key) // повтор с тем же ключом ничего не меняет
        prog.cardsReviewed += 1
        prog.minutes += prog.step
        card.due = false
      }
      return route.fulfill({ json: queueBody() })
    }
    if (method === 'POST') {
      const { lemma, pos } = route.request().postDataJSON() as { lemma: string; pos: string }
      const word = WORDS.find((w) => w.lemma === lemma && w.pos === pos)
      if (!word) return route.fulfill(apiError(404, 'word_not_found', 'Такого слова нет в словаре'))
      if (!CARD_SENTENCES[lemma]) return route.fulfill(apiError(409, 'no_examples', 'Для этого слова нет подходящих примеров, карточку не собрать'))
      const existing = cards.find((c) => c.lemma === lemma)
      if (existing) return route.fulfill({ status: 200, json: { cardId: existing.id, created: false } })
      cards.push({ id: nextCardId++, lemma, pos, level: word.level, due: true })
      return route.fulfill({ status: 201, json: { cardId: cards[cards.length - 1].id, created: true } })
    }
    return route.fulfill({ status: 404 })
  })

  // Упражнения в памяти: очередь фикстур, ответы, рейтинги тем.
  const ex = opts.exercises ?? {}
  const exState = { failAnswers: !!ex.failAnswers }
  const exQueue = [...(ex.queue ?? EXERCISES)]
  const answers: ExerciseAnswer[] = []
  const results = new Map<string, unknown>()
  const ratings = new Map<string, TopicRating>([
    ['grammar:articles', { topic: 'grammar:articles', rating: 1500, deviation: 350, attempts: 0 }],
    ['grammar:past_simple', { topic: 'grammar:past_simple', rating: 1500, deviation: 350, attempts: 0 }],
  ])
  const publicExercise = ({ answer: _a, accept: _c, word: _w, noExamples: _n, pairs: _p, repeat, ...rest }: ExerciseFixture) => ({ ...rest, ...(repeat ? { repeat: true } : {}) })

  // Банк ошибок в памяти: неверный ответ открывает ошибку, верный повтор двигает её, второй верный закрывает.
  const inDays = (d: number) => new Date(Date.now() + d * DAY_MS).toISOString()
  const mistakes: MockMistake[] = (opts.mistakes ?? []).map(({ dueInDays, ...m }) => ({ ...m, nextDue: inDays(dueInDays), firstWrongAt: inDays(-2) }))
  let closedThisWeek = opts.closedThisWeek ?? 0
  let nextMistakeId = 1000
  await page.route('**/api/v1/mistakes**', (route) => {
    const sorted = [...mistakes].sort((a, b) => a.nextDue.localeCompare(b.nextDue))
    const now = new Date().toISOString()
    return route.fulfill({
      json: {
        open: sorted.map(({ exerciseId: _e, ...m }) => m),
        counts: { open: mistakes.length, dueNow: mistakes.filter((m) => m.nextDue <= now).length, closedThisWeek },
      },
    })
  })
  const touchMistake = (fx: ExerciseFixture, given: string, correct: boolean) => {
    let m = mistakes.find((x) => x.exerciseId === fx.id)
    if (fx.repeat && m && correct) {
      m.streak += 1
      if (m.streak >= 2) {
        mistakes.splice(mistakes.indexOf(m), 1)
        closedThisWeek += 1
        return { status: 'closed', streak: m.streak, nextDue: inDays(3) }
      }
      m.nextDue = inDays(3)
      return { status: 'open', streak: m.streak, nextDue: m.nextDue }
    }
    if (correct) return undefined
    if (!m) {
      m = { id: nextMistakeId++, exerciseId: fx.id, topic: fx.topic, kind: fx.kind, prompt: fx.prompt, given, answer: fx.answer, streak: 0, nextDue: inDays(1), firstWrongAt: inDays(0) }
      mistakes.push(m)
    }
    m.given = given
    m.streak = 0
    m.nextDue = inDays(1)
    return { status: 'open', streak: 0, nextDue: m.nextDue }
  }

  await page.route('**/api/v1/exercises/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/exercises/topics')) {
      return route.fulfill({ json: { topics: [...ratings.values()] } })
    }
    if (path.endsWith('/exercises/next')) {
      if (ex.none || exQueue.length === 0) return route.fulfill(apiError(404, 'no_exercise', 'Подходящих упражнений пока нет'))
      return route.fulfill({ json: publicExercise(exQueue[0]) })
    }
    if (path.endsWith('/exercises/answer')) {
      const body = route.request().postDataJSON() as ExerciseAnswer
      answers.push(body)
      if (exState.failAnswers) return route.fulfill(apiError(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
      if (results.has(body.key)) return route.fulfill({ json: results.get(body.key) })
      const fx = exQueue.find((e) => e.id === body.exerciseId)
      if (!fx) return route.fulfill(apiError(404, 'not_found', 'Такого упражнения нет'))
      const norm = (t: string) => t.trim().replace(/\s+/g, ' ').replace(/[.!?]$/, '').toLowerCase()
      const correct =
        fx.kind === 'order' ? (fx.accept ?? [fx.answer]).some((a) => norm(a) === norm(body.given)) : body.given.trim().toLowerCase() === fx.answer.toLowerCase()
      // «Сопоставление»: разбор по парам, счёт дробный (доля верных).
      let pairResults: { word: string; given: string; answer: string; correct: boolean }[] | undefined
      let share = correct ? 1 : 0
      if (fx.kind === 'match' && fx.pairs) {
        const got = new Map(body.given.split(';').map((p) => p.split('=') as [string, string]))
        pairResults = fx.pairs.map((p) => ({ word: p.word, given: got.get(p.word) ?? '', answer: p.ru, correct: got.get(p.word) === p.ru }))
        share = pairResults.filter((p) => p.correct).length / pairResults.length
      }
      const matchCorrect = pairResults ? pairResults.every((p) => p.correct) : correct
      const r = ratings.get(fx.topic) ?? { topic: fx.topic, rating: 1500, deviation: 350, attempts: 0 }
      r.rating += pairResults ? Math.round(80 * share - 40) : correct ? 40 : -40
      r.deviation -= 20
      r.attempts += 1
      ratings.set(fx.topic, r)
      exQueue.splice(exQueue.indexOf(fx), 1)
      const mistake = fx.kind === 'match' ? undefined : touchMistake(fx, body.given, correct)
      prog.answers += 1
      prog.correct += matchCorrect ? 1 : 0
      prog.minutes += prog.step
      if (mistake && !matchCorrect) prog.opened += 1
      if (mistake?.status === 'closed') prog.closed += 1
      const result = { correct: matchCorrect, answer: fx.answer, translation: fx.translation, ratings: [{ ...r }], ...(fx.word ? { word: fx.word } : {}), ...(mistake ? { mistake } : {}), ...(pairResults ? { pairs: pairResults } : {}) }
      results.set(body.key, result)
      return route.fulfill({ json: result })
    }
    return route.fulfill({ status: 404 })
  })

  let suggestionVisible = true
  await page.route('**/api/v1/level/suggestion/dismiss', (route) => {
    if (opts.level?.dismissFails) return route.fulfill(apiError(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    if (opts.level?.dismissed) opts.level.dismissed.count++
    suggestionVisible = false
    return route.fulfill({ status: 204 })
  })
  await page.route('**/api/v1/level/suggestion', (route) =>
    route.fulfill({ json: opts.level?.suggestion && suggestionVisible ? { suggestion: opts.level.suggestion } : {} }),
  )

  await page.route('**/api/v1/milestones', (route) => {
    if (opts.milestones?.fail) return route.fulfill(apiError(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    const reached = new Set([...(opts.milestones?.reached ?? []), ...(opts.milestones?.newToday ?? [])])
    const today = new Date()
    const pad = (n: number) => String(n).padStart(2, '0')
    const todayIso = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`
    const plural = (n: number, a: string, b: string, c: string) => (n % 100 >= 11 && n % 100 <= 14 ? c : n % 10 === 1 ? a : n % 10 >= 2 && n % 10 <= 4 ? b : c)
    const defs: { id: string; kind: string; title: string; threshold: number }[] = [
      ...[3, 7, 14, 30, 100, 365].map((t) => ({ id: `streak-${t}`, kind: 'streak', title: `Серия ${t} ${plural(t, 'день', 'дня', 'дней')}`, threshold: t })),
      ...[1, 5, 10, 25, 50, 100, 250].map((t) => ({ id: `hours-${t}`, kind: 'hours', title: `${t} ${plural(t, 'час', 'часа', 'часов')} занятий`, threshold: t })),
      ...[100, 500, 1000].map((t) => ({ id: `answers-${t}`, kind: 'answers', title: `${t} ${plural(t, 'ответ', 'ответа', 'ответов')}`, threshold: t })),
      { id: 'first-card', kind: 'first', title: 'Первая карточка', threshold: 1 },
      { id: 'first-exercise', kind: 'first', title: 'Первое упражнение', threshold: 1 },
      { id: 'first-mistake-closed', kind: 'first', title: 'Первая закрытая ошибка', threshold: 1 },
    ]
    const milestones = defs.map((d) => {
      const isReached = reached.has(d.id)
      const value = isReached ? d.threshold : d.kind === 'first' ? 0 : Math.floor(d.threshold * 0.4)
      return {
        ...d,
        ...(isReached ? { reachedOn: opts.milestones?.newToday?.includes(d.id) ? todayIso : '2026-10-19' } : {}),
        progress: { value, target: d.threshold },
      }
    })
    const order: Record<string, number> = { streak: 0, hours: 1, answers: 2, first: 3 }
    const newToday = (opts.milestones?.newToday ?? [])
      .map((id) => defs.find((d) => d.id === id)!)
      .sort((a, b) => order[a.kind] - order[b.kind] || b.threshold - a.threshold)
      .map((d) => d.id)
    return route.fulfill({ json: { milestones, newToday } })
  })

  await page.route('**/api/v1/progress/overview', (route) => {
    if (opts.overview?.fail) return route.fulfill(apiError(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    const active = opts.overview?.activeDays ?? 0
    const perDay = opts.overview?.minutesPerDay ?? 0
    const pad = (n: number) => String(n).padStart(2, '0')
    const iso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
    const now = new Date()
    let cum = 0
    const daily = Array.from({ length: 90 }, (_, i) => {
      const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - (89 - i))
      const minutes = 89 - i < active ? perDay : 0
      cum += minutes
      return { day: iso(d), minutes, cumulativeHours: Math.round((cum / 60) * 100) / 100 }
    })
    const started = active > 0 ? iso(new Date(now.getFullYear(), now.getMonth(), now.getDate() - (active - 1))) : undefined
    return route.fulfill({
      json: {
        ...(started ? { startedOn: started } : {}),
        daysSinceStart: active,
        studiedMinutes: active * perDay,
        paceMinutesPerDay: Math.round(((Math.min(active, 14) * perDay) / 14) * 10) / 10,
        paceReady: active >= 14,
        daily,
      },
    })
  })

  await page.route('**/api/v1/progress/today', (route) => {
    if (opts.progress?.fail) return route.fulfill(apiError(500, 'internal', 'Внутренняя ошибка, попробуй ещё раз'))
    const minutes = Math.round(prog.minutes * 10) / 10
    return route.fulfill({
      json: {
        minutes,
        minimumDay: 10,
        goalMinutes: 15,
        doneToday: prog.minutes >= 10,
        streakDays: opts.progress?.streakDays ?? 3,
        ...(opts.progress?.freezeUsedOn ? { freezeUsedOn: opts.progress.freezeUsedOn } : {}),
        today: { answers: prog.answers, correct: prog.correct, cardsReviewed: prog.cardsReviewed, mistakesOpened: prog.opened, mistakesClosed: prog.closed },
      },
    })
  })

  return { puts, reviews, answers, exState, cardAdds }
}
