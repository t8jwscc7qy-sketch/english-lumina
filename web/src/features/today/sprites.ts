import type { PixelArt } from '@/lib/pixel'

type FlagKind = 'done' | 'target' | 'future'

const FILL: Record<FlagKind, string> = {
  done: 'var(--grass)',
  target: 'var(--sun)',
  future: 'var(--muted)',
}

/** Флажок на этапе: зелёный — пройден, жёлтый — цель, серый — ещё впереди. */
export function flagArt(kind: FlagKind): PixelArt {
  return {
    palette: { K: 'var(--pixel-ink)', F: FILL[kind] },
    rows: [
      '.KKKKKKKK',
      '.KFFFFFFK',
      '.KFFFFFFK',
      '.KFFFFFK.',
      '.KFFFFK..',
      '.KKKKK...',
      '.K.......',
      '.K.......',
      '.K.......',
      '.K.......',
    ],
  }
}
