import type { PixelArt } from '@/lib/pixel'

export type FoxMood = 'neutral' | 'happy' | 'thinking' | 'sleeping'

/** Левая половина лисёнка (12 клеток: 0 — край, 11 — середина); правая зеркалится. */
const HALF: string[] = [
  '...KK.......', // 0
  '..KOOK......',
  '..KOEOK.....',
  '..KOEEOK....',
  '..KOEEOOKKKK',
  '..KOEOOOOOOO', // 5
  '..KOOOOOOOOO',
  '.KOOOOOOOOOO',
  '.KOOOOOOOOOO',
  '.KOOOOOOOOOO', // 9  глаза
  '.KOOOOOOOOOO', // 10
  '.KOOOOOOOOOO', // 11
  '.KOOCCOOOOOO',
  '.KOCCCCCOOOO',
  '..KCCCCCCCCC', // 14 нос
  '..KCCCCCCCCC', // 15 рот
  '...KCCCCCCCC',
  '....KKCCCCCC',
  '...KBBBBBBBB', // 18 шарф
  '..KBBBBBBBBB',
  '..KOBbbbbbbb',
  '..KOOCCCCCCC',
  '..KOOCCCCCCC',
  '..KDDKCCCCCC',
  '...KKKKKKKKK',
]

/** Хвост справа (6 клеток), строки 14…24. */
const TAIL: string[] = [
  '....KK',
  '...KCK',
  '...KCK',
  '..KOOK',
  '..KOOK',
  '..KOOK',
  '.KOOK.',
  'KOOK..',
  'KOOK..',
  'KOOK..',
  'KKK...',
]
const TAIL_FROM = 14
/** Сон: буква Z голубым над хвостом, строки 1…5. */
const ZZZ: string[] = ['.BBB..', '...B..', '..B...', '.B....', '.BBB..']
const ZZZ_FROM = 1

function mirror(half: string): string {
  return half + [...half].reverse().join('')
}

/** Строки лица, которые зависят от настроения: номер строки → левая половина. */
function faceRows(mood: FoxMood): Record<number, string> {
  // нос общий: строки 13–14, середина
  const base: Record<number, string> = {
    13: '.KOCCCCCCCCK',
    14: '..KCCCCCCCCK',
  }
  const eyesOpen = {
    9: '.KOOOKWOOOOO',
    10: '.KOOOKKOOOOO',
    11: '.KOOOKKOOOOO',
  }
  switch (mood) {
    case 'happy':
      return {
        ...base,
        9: '.KOOOOKOOOOO',
        10: '.KOOOKOKOOOO',
        12: '.KORCCOOOOOO',
        15: '..KCCCCCCKKK',
        16: '...KCCCCCCKR',
      }
    case 'sleeping':
      return { ...base, 10: '.KOOOKOKOOOO', 11: '.KOOOOKOOOOO', 15: '..KCCCCCCCKC', 16: '...KCCCCCCCK' }
    case 'thinking':
      return { ...base, ...eyesOpen, 8: '.KOOODDDOOOO', 15: '..KCCCCCCCKK' }
    default:
      return { ...base, ...eyesOpen, 15: '..KCCCCCCCKC', 16: '...KCCCCCCCK' }
  }
}

export function foxArt(mood: FoxMood): PixelArt {
  const overrides = faceRows(mood)
  const rows = HALF.map((h, i) => mirror(overrides[i] ?? h))
  const full = rows.map((r, i) => {
    let t = i >= TAIL_FROM ? (TAIL[i - TAIL_FROM] ?? '......') : '......'
    if (mood === 'sleeping' && i >= ZZZ_FROM && i < ZZZ_FROM + ZZZ.length) t = ZZZ[i - ZZZ_FROM]
    return r.slice(0, 22) + t
  })
  return {
    palette: {
      K: 'var(--pixel-ink)',
      O: 'var(--fox)',
      D: 'var(--fox-dark)',
      C: 'var(--fox-cream)',
      E: 'var(--fox-ear)',
      B: 'var(--sky)',
      b: 'var(--sky-deep)',
      W: '#ffffff',
      R: 'var(--coral-soft)',
      N: 'var(--ink)',
    },
    rows: full,
  }
}
