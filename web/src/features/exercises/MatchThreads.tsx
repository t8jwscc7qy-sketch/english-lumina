import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import { pairColor } from './match'
import { threadPath } from './matchFx'

/** Цвета нитей совпадают с цветами рамок пар. */
const STROKE = ['var(--sky)', 'var(--grass)', 'var(--sun)', 'var(--coral-soft)', 'var(--bridge)'] as const

type Props = {
  /** Контейнер, относительно которого считаются координаты (position: relative). */
  container: HTMLElement | null
  wordEls: RefObject<(HTMLElement | null)[]>
  optionEls: RefObject<(HTMLElement | null)[]>
  words: string[]
  options: string[]
  pairs: (number | null)[]
  /** После ответа нить окрашена по верности: true — верно, false — неверно. */
  verdicts?: (boolean | undefined)[]
}

/**
 * Пиксельные нити между соединёнными плитками: ступенчатый путь по сетке 4px поверх колонок, только рисунок.
 * Какие нити есть, решает React; координаты считаются по раскладке после отрисовки и пишутся в атрибут `d`.
 * Появление анимирует CSS (класс m-thread), разрыв мгновенный: действие частое.
 */
export function MatchThreads({ container, wordEls, optionEls, words, options, pairs, verdicts }: Props) {
  const pathEls = useRef<(SVGPathElement | null)[]>([])
  const [tick, setTick] = useState(0)

  // Размеры меняются (окно, шрифт, перенос строк): нити пересчитываются.
  useEffect(() => {
    if (!container || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => setTick((t) => t + 1))
    ro.observe(container)
    return () => ro.disconnect()
  }, [container])

  useLayoutEffect(() => {
    if (!container) return
    const box = container.getBoundingClientRect()
    pairs.forEach((o, i) => {
      const path = pathEls.current[i]
      const w = wordEls.current?.[i]
      const t = o === null ? null : optionEls.current?.[o]
      if (!path || !w || !t) return
      const a = w.getBoundingClientRect()
      const b = t.getBoundingClientRect()
      path.setAttribute(
        'd',
        threadPath(
          { x: Math.round(a.right - box.left), y: Math.round(a.top + a.height / 2 - box.top) },
          { x: Math.round(b.left - box.left), y: Math.round(b.top + b.height / 2 - box.top) },
          i,
        ),
      )
    })
  }, [container, wordEls, optionEls, pairs, verdicts, tick])

  return (
    <svg aria-hidden className="pointer-events-none absolute inset-0 size-full overflow-visible" shapeRendering="crispEdges">
      {pairs.map((o, i) => {
        if (o === null) return null
        const v = verdicts?.[i]
        const stroke = v === undefined ? STROKE[pairColor(i)] : v ? 'var(--grass-text)' : 'var(--coral)'
        return (
          <path
            key={`${words[i]}:${options[o]}`}
            ref={(el) => {
              pathEls.current[i] = el
            }}
            className="m-thread"
            stroke={stroke}
            strokeWidth={4}
            fill="none"
          />
        )
      })}
    </svg>
  )
}
