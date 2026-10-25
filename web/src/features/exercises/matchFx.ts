/** Чистые помощники анимаций «сопоставления»: искры, конфетти, ступенчатая нить и память об источнике ввода. */

export type Point = { x: number; y: number }

const SPARK_RADIUS = 12
const GRID = 4

/** Искры вокруг верной пары: n точек на окружности радиуса 12px; seed поворачивает рисунок. */
export function sparkOffsets(seed: number, n: number): { dx: number; dy: number }[] {
  const turn = ((Math.abs(Math.trunc(seed)) * 37) % 360) * (Math.PI / 180)
  return Array.from({ length: n }, (_, k) => {
    const a = turn + (2 * Math.PI * k) / n
    return { dx: Math.round(SPARK_RADIUS * Math.cos(a)), dy: Math.round(SPARK_RADIUS * Math.sin(a)) }
  })
}

function lcg(seed: number): () => number {
  let s = (Math.abs(Math.trunc(seed)) * 2654435761 + 12345) >>> 0
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0
    return s
  }
}

/** Конфетти: x и y в процентах области (y — начальная высота), задержка до 200 мс, цвет из пяти. */
export function confetti(seed: number, n: number): { x: number; y: number; delay: number; color: 0 | 1 | 2 | 3 | 4 }[] {
  const next = lcg(seed)
  return Array.from({ length: n }, (_, i) => ({
    x: next() % 101,
    y: next() % 31,
    delay: next() % 201,
    // первые пять — все цвета подряд, чтобы набор не вышел одноцветным
    color: (i < 5 ? i : next() % 5) as 0 | 1 | 2 | 3 | 4,
  }))
}

const snap = (v: number) => Math.round(v / GRID) * GRID

/**
 * Нить от правого края слова (from) к левому краю перевода (to): горизонталь, вертикаль по сетке 4px, горизонталь.
 * lane (0…4) сдвигает вертикаль, чтобы пять нитей в одной щели не слипались; вертикаль не выходит за щель.
 */
export function threadPath(from: Point, to: Point, lane: number): string {
  if (from.y === to.y) return `M${from.x} ${from.y} H${to.x}`
  const lo = Math.ceil(from.x / GRID) * GRID
  const hi = Math.floor(to.x / GRID) * GRID
  let xm = snap((from.x + to.x) / 2) + (lane - 2) * GRID
  xm = lo > hi ? snap((from.x + to.x) / 2) : Math.min(hi, Math.max(lo, xm))
  return `M${from.x} ${from.y} H${xm} V${to.y} H${to.x}`
}

// Источник ввода: последнее действие пользователя. Клавиатурные пути идут без анимаций.
let last: 'keyboard' | 'pointer' = 'pointer'
let tracking = false

export function trackInput(): void {
  if (tracking || typeof window === 'undefined') return
  tracking = true
  window.addEventListener('keydown', () => (last = 'keyboard'), true)
  window.addEventListener('pointerdown', () => (last = 'pointer'), true)
}

export const lastInput = (): 'keyboard' | 'pointer' => last

/** Системная настройка «уменьшить движение»; без matchMedia (тесты) — обычное движение. */
export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(prefers-reduced-motion: reduce)').matches
    : false
}
