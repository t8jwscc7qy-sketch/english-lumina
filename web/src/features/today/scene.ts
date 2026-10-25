import type { PixelArt } from '@/lib/pixel'

/** Пиксельная сцена «лагерь у дороги»: чистая сборка клеток по времени суток (без DOM, чтобы тестировать). */

export type Phase = 'morning' | 'day' | 'dusk' | 'night'

export const SCENE_W = 210
export const SCENE_H = 51
/** Где в клетках сцены стоят лисёнок (левый верхний угол) и костёр. */
export const FOX_AT = { x: 78, y: 25 }
/** Костёр: середина по x и нижняя граница (клетка под поленьями), спрайты разной высоты стоят на ней. */
export const FIRE_CX = 121
export const FIRE_BASE_Y = 48

export interface SceneRect {
  x: number
  y: number
  w: number
  h: number
  fill: string
  opacity?: number
}

/** Час суток → время дня сцены. Ночь 21:00–5:00 совпадает со «сном» лисёнка (до 5 утра). */
export function phaseOfHour(hour: number): Phase {
  const h = ((Math.floor(hour) % 24) + 24) % 24
  if (h < 5 || h >= 21) return 'night'
  if (h < 9) return 'morning'
  if (h < 17) return 'day'
  return 'dusk'
}

interface Look {
  sky: string[]
  far: string
  mid: string
  grassTop: string
  ground: string
  road: string
  roadEdge: string
  tree: string
  trunk: string
  cloud: string | null
  glow: boolean
}

const LOOK: Record<Phase, Look> = {
  day: {
    sky: ['#4aa8e8', '#5fb6ee', '#78c4f2', '#92d1f5', '#acdcf7', '#c4e7f9'],
    far: '#8fc3d8', mid: '#6fb08a', grassTop: '#63c27f', ground: '#4aa86a',
    road: '#e0bd86', roadEdge: '#b98d58', tree: '#2f7d52', trunk: '#6b4326', cloud: '#ffffff', glow: false,
  },
  morning: {
    sky: ['#7a86d6', '#98a0e4', '#d89fc0', '#f5aaa0', '#fbc59a', '#fdddb0'],
    far: '#9a96c8', mid: '#7fae96', grassTop: '#7bc58c', ground: '#5ba878',
    road: '#e6c08e', roadEdge: '#bf9362', tree: '#37805a', trunk: '#6b4326', cloud: '#ffe6ea', glow: true,
  },
  dusk: {
    sky: ['#27256b', '#4e3584', '#8e4590', '#d2606f', '#f08a5c', '#fbb360'],
    far: '#6c3f7e', mid: '#3f5b6e', grassTop: '#4a8a62', ground: '#376e52',
    road: '#b88a60', roadEdge: '#8c6240', tree: '#1f5a42', trunk: '#4a2e1c', cloud: '#f4a58f', glow: true,
  },
  night: {
    sky: ['#090c22', '#0e1233', '#141a46', '#1a2258', '#212b69', '#2a367a'],
    far: '#1b2255', mid: '#1a3350', grassTop: '#2a6b55', ground: '#1e5444',
    road: '#5b5a6e', roadEdge: '#43425a', tree: '#134035', trunk: '#2d1d12', cloud: '#232b66', glow: true,
  },
}

const SUN_ROWS = [3, 5, 7, 7, 7, 5, 3]
const CLOUD = ['..XXX...', '.XXXXXX.', 'XXXXXXXX']
const PINE = ['...T...', '..TTT..', '..TTT..', '.TTTTT.', '.TTTTT.', 'TTTTTTT', '...N...', '...N...']

export const FIRE_PALETTE: Record<string, string> = {
  Y: '#ffd23f', O: '#f08a3c', R: '#e8553a', L: '#7a4a2a', S: '#fff3b0',
}

export type FireLevel = 0 | 1 | 2 | 3 | 4

/** Ступень костра: по серии (угли, малый, средний, большой, с искрами); выполненный минимум дня даёт ещё ступень, если серия есть. */
export function fireLevel(streakDays: number, doneToday: boolean): FireLevel {
  const d = Number.isFinite(streakDays) ? Math.floor(streakDays) : 0
  if (d <= 0) return 0
  const base = d >= 14 ? 4 : d >= 7 ? 3 : d >= 3 ? 2 : 1
  return Math.min(4, base + (doneToday ? 1 : 0)) as FireLevel
}

/** Поленьев в поленнице: по одному на день серии, не больше 10. */
export function logCount(streakDays: number): number {
  const d = Number.isFinite(streakDays) ? Math.floor(streakDays) : 0
  return Math.min(10, Math.max(0, d))
}

const FIRE: Record<FireLevel, [string[], string[]]> = {
  0: [
    ['.......', '..R.R..', 'LLLLLLL', '.LLLLL.'],
    ['.......', '.R...R.', 'LLLLLLL', '.LLLLL.'],
  ],
  1: [
    ['...Y...', '..YOY..', '..ORO..', 'LLLLLLL', '.LLLLL.'],
    ['..Y....', '..YOY..', '..ORO..', 'LLLLLLL', '.LLLLL.'],
  ],
  2: [
    ['...Y...', '..YOY..', '..YOO..', '.YOOOY.', '.YORRO.', '..ORRO.', 'LLLLLLL', '.LLLLL.'],
    ['....Y..', '..YOY..', '.YOOY..', '.YOOOY.', '.OORRY.', '..ORRO.', 'LLLLLLL', '.LLLLL.'],
  ],
  3: [
    ['....Y....', '...YOY...', '...YOO...', '..YOOOY..', '.YOOOOOY.', '.YOORROY.', '..OORROO.', '..ORRRO..', 'LLLLLLLLL', '.LLLLLLL.'],
    ['.....Y...', '...YOY...', '..YOOY...', '..YOOOY..', '.YOOOOOY.', '.OOORROY.', '..OORROO.', '..ORRRO..', 'LLLLLLLLL', '.LLLLLLL.'],
  ],
  4: [
    ['.S.....S.', '....S....', '....Y....', '...YOY...', '...YOO...', '..YOOOY..', '.YOOOOOY.', '.YOORROY.', '..OORROO.', '..ORRRO..', 'LLLLLLLLL', '.LLLLLLL.'],
    ['.....S...', 'S.......S', '....Y....', '...YOY...', '..YOOY...', '..YOOOY..', '.YOOOOOY.', '.OOORROY.', '..OORROO.', '..ORRRO..', 'LLLLLLLLL', '.LLLLLLL.'],
  ],
}

/** Два кадра пламени ступени. */
export function fireFrames(level: FireLevel): [PixelArt, PixelArt] {
  const [a, b] = FIRE[level]
  return [
    { palette: FIRE_PALETTE, rows: a },
    { palette: FIRE_PALETTE, rows: b },
  ]
}

const GLOW: Record<FireLevel, [number, number][]> = {
  0: [],
  1: [[5, 0.12]],
  2: [[8, 0.1], [5, 0.16]],
  3: [[10, 0.1], [7, 0.14], [4, 0.2]],
  4: [[12, 0.1], [9, 0.14], [6, 0.18], [3, 0.22]],
}

/** Что показывает сцена о занятиях пользователя. Всё необязательно: без данных рисуется спокойный пейзаж. */
export interface SceneExtras {
  fireLevel?: FireLevel
  /** Поленьев в поленнице (0…10). */
  logs?: number
  /** Достигнутых вех вдоль дороги (рисуются последние 7). */
  posts?: number
  /** Есть ли ещё недостигнутая веха: она стоит вдали серой. */
  nextPost?: boolean
}

/** Псевдослучайные звёзды: одно зерно, всегда одни и те же места. */
function stars(count: number, maxY: number): [number, number][] {
  let s = 12345
  const next = () => (s = (s * 1103515245 + 12345) & 0x7fffffff)
  const out: [number, number][] = []
  while (out.length < count) {
    const x = next() % SCENE_W
    const y = next() % maxY
    if (x > 150 && x < 186 && y < 22) continue // место луны
    out.push([x, y])
  }
  return out
}

const GROUND_Y = 42
const ROAD_MID_TOP = 188

export function buildScene(phase: Phase, extras: SceneExtras = {}): SceneRect[] {
  const { fireLevel: fire = 2, logs = 0, posts = 0, nextPost = false } = extras
  const look = LOOK[phase]
  const grid: (string | null)[][] = Array.from({ length: SCENE_H }, () => Array<string | null>(SCENE_W).fill(null))
  const put = (x: number, y: number, c: string) => {
    if (x >= 0 && x < SCENE_W && y >= 0 && y < SCENE_H) grid[y][x] = c
  }

  // Небо полосами; на стыке полос шахматка, чтобы переход был пиксельным.
  const band = Math.ceil(GROUND_Y / look.sky.length)
  for (let y = 0; y < SCENE_H; y++) {
    const i = Math.min(look.sky.length - 1, Math.floor(y / band))
    for (let x = 0; x < SCENE_W; x++) {
      const prev = look.sky[Math.max(0, i - 1)]
      put(x, y, y % band === 0 && i > 0 && (x + y) % 2 === 0 ? prev : look.sky[i])
    }
  }

  if (phase === 'night' || phase === 'dusk') {
    for (const [x, y] of stars(phase === 'night' ? 34 : 10, phase === 'night' ? 16 : 7)) put(x, y, '#f7f7ff')
  }

  // Солнце или луна.
  const cx = phase === 'dusk' ? 162 : 168
  const cy = phase === 'dusk' ? 26 : phase === 'morning' ? 22 : 10
  const body = phase === 'night' ? '#f4f1de' : phase === 'dusk' ? '#ffb35a' : '#ffe27a'
  SUN_ROWS.forEach((w, r) => {
    for (let k = 0; k < w; k++) put(cx - Math.floor(w / 2) + k, cy + r, body)
  })
  if (phase === 'night') {
    put(cx - 1, cy + 2, '#d9d6c3')
    put(cx + 1, cy + 4, '#d9d6c3')
  }

  // Облака.
  if (look.cloud) {
    for (const [ox, oy] of [[18, 6], [99, 3], [186, 16]] as const) {
      CLOUD.forEach((row, r) => [...row].forEach((ch, k) => ch === 'X' && put(ox + k, oy + r, look.cloud!)))
    }
  }

  // Дальние и средние холмы.
  for (let x = 0; x < SCENE_W; x++) {
    const far = 31 + Math.round(3 * Math.sin(x / 16) + 2 * Math.sin(x / 6.5))
    for (let y = far; y < SCENE_H; y++) put(x, y, look.far)
    const mid = 37 + Math.round(2.5 * Math.sin((x + 30) / 13) + 1.5 * Math.sin(x / 4.4))
    for (let y = mid; y < SCENE_H; y++) put(x, y, look.mid)
  }

  // Земля: два верхних ряда светлее.
  for (let x = 0; x < SCENE_W; x++) {
    for (let y = GROUND_Y; y < SCENE_H; y++) put(x, y, y < GROUND_Y + 2 ? look.grassTop : look.ground)
  }
  for (let x = 3; x < SCENE_W; x += 7) put(x, GROUND_Y - 1, look.grassTop)

  // Дорога уходит к холму: трапеция со смещением влево к низу.
  for (let y = GROUND_Y - 4; y < SCENE_H; y++) {
    const t = (y - (GROUND_Y - 4)) / (SCENE_H - 1 - (GROUND_Y - 4))
    const mid = Math.round(ROAD_MID_TOP - 24 * t)
    const half = Math.round(2 + 17 * t)
    for (let x = mid - half; x <= mid + half; x++) {
      put(x, y, x === mid - half || x === mid + half ? look.roadEdge : look.road)
    }
  }

  // Сосны по краям.
  for (const [px, base] of [[6, 45], [20, 47], [34, 45], [54, 46], [190, 46], [202, 48]] as const) {
    PINE.forEach((row, r) =>
      [...row].forEach((ch, k) => {
        if (ch === 'T') put(px + k, base - PINE.length + 1 + r, look.tree)
        if (ch === 'N') put(px + k, base - PINE.length + 1 + r, look.trunk)
      }),
    )
  }

  // Поленница справа от костра: брёвна пирамидой 4-3-2-1, по одному на день серии.
  const LOG_SPOTS: [number, number][] = [
    [126, 46], [130, 46], [134, 46], [138, 46],
    [128, 44], [132, 44], [136, 44],
    [130, 42], [134, 42],
    [132, 40],
  ]
  LOG_SPOTS.slice(0, logCount(logs)).forEach(([lx, ly]) => {
    put(lx, ly, '#8f5a35'); put(lx + 1, ly, '#8f5a35'); put(lx + 2, ly, '#c48a52')
    put(lx, ly + 1, '#7a4a2a'); put(lx + 1, ly + 1, '#7a4a2a'); put(lx + 2, ly + 1, '#c48a52')
  })

  // Вехи стоят рядком на дороге впереди: старые ближе к лагерю, новые дальше по пути, серая (ещё не достигнутая) последней.
  const ink = phase === 'night' ? '#6e78b8' : '#1e2347'
  const post = (slot: number, stripe: [string, string], cap: string) => {
    const rows = ['C', 'A', 'B', 'A', 'B', 'A', 'K']
    const px = 144 + slot * 6
    rows.forEach((kind, i) => {
      const y = SCENE_H - rows.length + i
      const mid = kind === 'C' ? cap : kind === 'A' ? stripe[0] : kind === 'B' ? stripe[1] : ink
      put(px, y, ink); put(px + 1, y, mid); put(px + 2, y, mid); put(px + 3, y, ink)
    })
  }
  const shown = Math.min(7, Math.max(0, Number.isFinite(posts) ? Math.floor(posts) : 0))
  for (let i = 0; i < shown; i++) post(i, ['#ffd23f', '#f7f7ff'], '#4cc9f0')
  if (nextPost) post(shown, ['#9aa3c7', '#9aa3c7'], '#9aa3c7')

  // Клетки → прямоугольники по горизонтальным сериям одного цвета.
  const rects: SceneRect[] = []
  grid.forEach((row, y) => {
    let x = 0
    while (x < SCENE_W) {
      const c = row[x]
      if (!c) {
        x++
        continue
      }
      let e = x + 1
      while (e < SCENE_W && row[e] === c) e++
      rects.push({ x, y, w: e - x, h: 1, fill: c })
      x = e
    }
  })

  // Свет костра ступенчатым ромбом поверх сцены (вечером, ночью, утром); сильнее у большого огня.
  if (look.glow) {
    const gx = FIRE_CX
    const gy = FIRE_BASE_Y - 4
    for (const [r, opacity] of GLOW[fire]) {
      for (let dy = -r; dy <= r; dy++) {
        const hw = (r - Math.abs(dy)) * 2
        const y = gy + dy
        if (y >= 0 && y < SCENE_H) rects.push({ x: Math.max(0, gx - hw), y, w: Math.min(SCENE_W, gx + hw + 1) - Math.max(0, gx - hw), h: 1, fill: '#ffb347', opacity })
      }
    }
  }
  return rects
}
