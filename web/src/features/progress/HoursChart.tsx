import { useState } from 'react'
import { formatHoursMinutes, formatRange, formatShortDay } from './progress'

type Point = { day: string; minutes: number; cumulativeHours: number }

const TICK_EVERY = 14 // подпись оси времени раз в 2 недели
const NARROW_TICK_EVERY = 42 // на узком экране (до sm) подписи реже, иначе наезжают друг на друга
/** Нижняя граница шкалы: пока занятий почти нет, ось не растягивается на доли минуты. */
const MIN_SCALE_HOURS = 0.25
const VIEW_W = 1000
const VIEW_H = 200

/** Подпись оси: до часа в минутах, до 10 часов с десятыми, дальше целые часы. */
function formatAxisHours(hours: number): string {
  if (hours < 1) return formatHoursMinutes(hours * 60)
  if (hours < 10) return `${(Math.round(hours * 10) / 10).toString().replace('.', ',')} ч`
  return `${Math.round(hours)} ч`
}

/**
 * Линия накопленных часов. Линия, сетка и полоса «нужно» рисуются SVG, подписи — обычным HTML-текстом, чтобы
 * размер шрифта не сжимался вместе с графиком на узком экране. Для читалок график скрыт (`aria-hidden`):
 * те же числа лежат таблицей рядом со страницей. Полоса «нужно» показывается, только когда укладывается в шкалу
 * данных: иначе у начинающего линия прижата к нулю и ничего не показывает.
 */
export function HoursChart({ points, neededMin, neededMax }: { points: Point[]; neededMin: number; neededMax: number }) {
  const [hover, setHover] = useState<number | null>(null)
  const n = points.length
  if (n === 0) return null

  const last = points[n - 1].cumulativeHours
  const yMax = Math.max(last * 1.15, MIN_SCALE_HOURS)
  const x = (i: number) => (n === 1 ? 0 : (i / (n - 1)) * VIEW_W)
  const y = (h: number) => VIEW_H - (Math.min(h, yMax) / yMax) * VIEW_H
  const line = points.map((p, i) => `${x(i).toFixed(1)},${y(p.cumulativeHours).toFixed(1)}`).join(' ')
  const showBand = neededMin <= yMax
  const ticks = Array.from({ length: Math.ceil(n / TICK_EVERY) }, (_, k) => k * TICK_EVERY).filter((i) => i < n)

  function onMove(e: React.PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    if (rect.width <= 0) return
    const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    setHover(Math.round(ratio * (n - 1)))
  }

  const hp = hover === null ? null : points[hover]
  return (
    <div aria-hidden="true" className="flex flex-col gap-1 pl-14 pr-3 pb-6">
      <div
        data-testid="hours-surface"
        className="relative h-48 touch-pan-y"
        onPointerMove={onMove}
        onPointerLeave={() => setHover(null)}
      >
        <svg viewBox={`0 0 ${VIEW_W} ${VIEW_H}`} preserveAspectRatio="none" className="absolute inset-0 h-full w-full overflow-visible">
          {[0, 0.5, 1].map((f) => (
            <line key={f} x1={0} x2={VIEW_W} y1={y(yMax * f)} y2={y(yMax * f)} className="stroke-border" strokeWidth={1} vectorEffect="non-scaling-stroke" />
          ))}
          {showBand && (
            <rect
              data-testid="needed-band"
              x={0}
              width={VIEW_W}
              y={y(neededMax)}
              height={Math.max(0, y(neededMin) - y(neededMax))}
              className="fill-muted-foreground/15"
            />
          )}
          <polyline
            data-testid="hours-line"
            points={line}
            fill="none"
            className="stroke-foreground dark:stroke-sky"
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
            vectorEffect="non-scaling-stroke"
          />
          {hover !== null && (
            <line x1={x(hover)} x2={x(hover)} y1={0} y2={VIEW_H} className="stroke-muted-foreground" strokeWidth={1} vectorEffect="non-scaling-stroke" />
          )}
        </svg>

        {[0, 0.5, 1].map((f) => (
          <span key={f} className="absolute right-full whitespace-nowrap pr-2 text-xs tabular-nums text-muted-foreground" style={{ top: `${(1 - f) * 100}%`, transform: 'translateY(-50%)' }}>
            {formatAxisHours(yMax * f)}
          </span>
        ))}
        {showBand && (
          <span className="absolute left-1 text-xs text-muted-foreground" style={{ top: `${(y(neededMax) / VIEW_H) * 100}%` }}>
            {`нужно ${formatRange(neededMin, neededMax)}`}
          </span>
        )}
        <span
          className="absolute right-0 whitespace-nowrap text-xs font-semibold tabular-nums text-foreground"
          style={{ top: `${(y(last) / VIEW_H) * 100}%`, transform: 'translateY(-130%)' }}
        >
          {formatHoursMinutes(last * 60)}
        </span>

        {hp && hover !== null && (
          <div
            role="tooltip"
            className="pointer-events-none absolute z-10 rounded-lg border border-border bg-card px-3 py-2 text-xs shadow-sm"
            style={{ left: `${(x(hover) / VIEW_W) * 100}%`, top: 0, transform: hover > n / 2 ? 'translateX(-105%)' : 'translateX(5%)' }}
          >
            <p className="font-semibold">{formatShortDay(hp.day)}</p>
            <p className="tabular-nums">{`${formatHoursMinutes(hp.cumulativeHours * 60)} всего`}</p>
            <p className="tabular-nums text-muted-foreground">{`+${formatHoursMinutes(hp.minutes)} за день`}</p>
          </div>
        )}

        {ticks.map((i) => (
          <span
            key={i}
            className={`absolute top-full whitespace-nowrap pt-1.5 text-xs tabular-nums text-muted-foreground ${i % NARROW_TICK_EVERY === 0 ? '' : 'hidden sm:block'}`}
            style={{ left: `${(x(i) / VIEW_W) * 100}%`, transform: i === 0 ? 'none' : 'translateX(-50%)' }}
          >
            {formatShortDay(points[i].day)}
          </span>
        ))}
      </div>
    </div>
  )
}
