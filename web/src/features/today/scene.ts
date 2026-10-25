/** Пиксельная сцена «лагерь у дороги»: чистая сборка клеток по времени суток (без DOM, чтобы тестировать). */

export type Phase = 'morning' | 'day' | 'dusk' | 'night'

export const SCENE_W = 210
export const SCENE_H = 51
/** Где в клетках сцены стоят лисёнок (левый верхний угол) и костёр. */
export const FOX_AT = { x: 78, y: 25 }
export const FIRE_AT = { x: 118, y: 40 }

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

export const FIRE_FRAMES: string[][] = [
  ['...Y...', '..YOY..', '..YOO..', '.YOOOY.', '.YORRO.', '..ORRO.', 'LLLLLLL', '.LLLLL.'],
  ['....Y..', '..YOY..', '.YOOY..', '.YOOOY.', '.OORRY.', '..ORRO.', 'LLLLLLL', '.LLLLL.'],
]
export const FIRE_PALETTE: Record<string, string> = {
  Y: '#ffd23f', O: '#f08a3c', R: '#e8553a', L: '#7a4a2a',
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

export function buildScene(phase: Phase): SceneRect[] {
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
    const mid = Math.round(176 - 26 * t)
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

  // Свет костра ступенчатым ромбом поверх сцены (вечером, ночью, утром).
  if (look.glow) {
    const gx = FIRE_AT.x + 3
    const gy = FIRE_AT.y + 4
    for (const [r, opacity] of [[8, 0.1], [5, 0.16]] as const) {
      for (let dy = -r; dy <= r; dy++) {
        const hw = (r - Math.abs(dy)) * 2
        const y = gy + dy
        if (y >= 0 && y < SCENE_H) rects.push({ x: gx - hw, y, w: hw * 2 + 1, h: 1, fill: '#ffb347', opacity })
      }
    }
  }
  return rects
}
