import { useId } from 'react'
import { cn } from '@/lib/utils'

export type FoxMood = 'neutral' | 'happy' | 'thinking' | 'sleeping'

interface FoxProps {
  /** Ширина и высота в пикселях. */
  size?: number
  mood?: FoxMood
  className?: string
  /** Если задан, рисунок читается скринридером; иначе он декоративный. */
  title?: string
}

const INK = 'var(--ink)'

/** Глаза и брови: единственное, что меняется между настроениями (плюс рот и румянец). */
function Eyes({ mood }: { mood: FoxMood }) {
  if (mood === 'happy') {
    return (
      <g fill="none" stroke={INK} strokeWidth="3.2" strokeLinecap="round">
        <path d="M39 55 Q45 46 51 55" />
        <path d="M69 55 Q75 46 81 55" />
      </g>
    )
  }
  if (mood === 'sleeping') {
    return (
      <g fill="none" stroke={INK} strokeWidth="3" strokeLinecap="round">
        <path d="M39 52 Q45 58 51 52" />
        <path d="M69 52 Q75 58 81 52" />
      </g>
    )
  }
  const look = mood === 'thinking'
  return (
    <>
      <g className="fox-blink">
        <ellipse cx="45" cy={look ? 51 : 52} rx="4.2" ry="5.4" fill={INK} />
        <ellipse cx="75" cy={look ? 51 : 52} rx="4.2" ry="5.4" fill={INK} />
        <circle cx={look ? 47 : 46.6} cy={look ? 48.6 : 50} r="1.6" fill="#fff" />
        <circle cx={look ? 77 : 76.6} cy={look ? 48.6 : 50} r="1.6" fill="#fff" />
      </g>
      {look && (
        <g fill="none" stroke="var(--fox-dark)" strokeWidth="2.6" strokeLinecap="round">
          <path d="M38 43 L51 41" />
          <path d="M69 39 L82 42" />
        </g>
      )}
    </>
  )
}

function Mouth({ mood }: { mood: FoxMood }) {
  if (mood === 'happy') {
    return (
      <>
        <path
          d="M51 71 Q60 87 69 71 Z"
          fill={INK}
          strokeLinejoin="round"
          stroke={INK}
          strokeWidth="2"
        />
        <ellipse cx="60" cy="78.5" rx="4.6" ry="2.7" fill="var(--coral-soft)" />
      </>
    )
  }
  if (mood === 'thinking') {
    return (
      <path
        d="M54 75 Q60 73 66 76"
        fill="none"
        stroke={INK}
        strokeWidth="2"
        strokeLinecap="round"
      />
    )
  }
  return (
    <path
      d="M53 73 Q56.5 77 60 73 Q63.5 77 67 73"
      fill="none"
      stroke={INK}
      strokeWidth="2"
      strokeLinecap="round"
    />
  )
}

/**
 * Маскот «Вёрст»: лисёнок-путник с шарфом и полосатой вехой в лапе. Векторный, цвета из токенов
 * (--fox*, в обеих темах), четыре настроения. Дышит и моргает; при «уменьшить движение» стоит.
 */
export function Fox({ size = 120, mood = 'neutral', className, title }: FoxProps) {
  const uid = useId().replace(/:/g, '')
  const body = `${uid}-body`
  const stripes = `${uid}-post`
  return (
    <svg
      viewBox="0 0 120 120"
      width={size}
      height={size}
      role={title ? 'img' : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
      className={cn('shrink-0 overflow-visible', className)}
    >
      {title && <title>{title}</title>}
      <defs>
        <linearGradient id={body} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" style={{ stopColor: 'var(--fox-light)' }} />
          <stop offset="1" style={{ stopColor: 'var(--fox)' }} />
        </linearGradient>
        <clipPath id={stripes}>
          <rect x="9" y="60" width="10" height="54" rx="3.5" />
        </clipPath>
      </defs>

      {/* Тень на земле */}
      <ellipse cx="60" cy="115" rx="36" ry="4" fill={INK} opacity="0.16" />

      {/* Хвост */}
      <g className="fox-tail">
        <path
          d="M80 104 C108 110 122 86 112 62 C108 52 99 50 94 55 C101 68 94 84 76 90 Z"
          fill={`url(#${body})`}
        />
        <path
          d="M112 62 C108 52 99 50 94 55 C97 60 99 66 99 72 C107 74 114 70 112 62 Z"
          fill="var(--fox-cream)"
        />
      </g>

      {/* Веха в лапе: полосатый столбик */}
      <g clipPath={`url(#${stripes})`}>
        <rect x="9" y="60" width="10" height="54" fill="#f7f7ff" />
        <rect x="9" y="70" width="10" height="10" fill="#ffd23f" />
        <rect x="9" y="90" width="10" height="10" fill="#ffd23f" />
        <rect x="9" y="110" width="10" height="10" fill="#ffd23f" />
      </g>
      <rect x="9" y="60" width="10" height="54" rx="3.5" fill="none" stroke="#1e2347" strokeOpacity="0.45" strokeWidth="1.2" />
      <rect x="7.5" y="55" width="13" height="7" rx="3.5" fill="var(--sky)" />

      <g className="fox-breathe">
        {/* Тело */}
        <path d="M34 114 C32 92 44 78 60 78 C76 78 88 92 86 114 Z" fill={`url(#${body})`} />
        <path d="M47 114 C46 98 52 88 60 88 C68 88 74 98 73 114 Z" fill="var(--fox-cream)" />
        <ellipse cx="46" cy="113" rx="9" ry="4.2" fill="var(--fox-dark)" />
        <ellipse cx="74" cy="113" rx="9" ry="4.2" fill="var(--fox-dark)" />
        {/* Лапа с вехой */}
        <ellipse cx="22" cy="86" rx="8" ry="6.8" fill="var(--fox-dark)" />

        {/* Шарф */}
        <path d="M70 90 L82 93 L79 108 L67 105 Z" fill="var(--sky-deep)" />
        <path d="M38 79 C50 90 70 90 82 79 L84 88 C70 99 50 99 36 88 Z" fill="var(--sky)" />

        {/* Уши */}
        <g strokeLinejoin="round" strokeWidth="7">
          <path d="M27 42 L31 12 L55 31 Z" fill={`url(#${body})`} stroke={`url(#${body})`} />
          <path d="M93 42 L89 12 L65 31 Z" fill={`url(#${body})`} stroke={`url(#${body})`} />
        </g>
        <g strokeLinejoin="round" strokeWidth="4">
          <path d="M34 33 L35 19 L47 28 Z" fill="var(--fox-ear)" stroke="var(--fox-ear)" />
          <path d="M86 33 L85 19 L73 28 Z" fill="var(--fox-ear)" stroke="var(--fox-ear)" />
        </g>

        {/* Голова */}
        <path
          d="M20 54 C20 33 38 26 60 26 C82 26 100 33 100 54 C100 71 82 84 60 84 C38 84 20 71 20 54 Z"
          fill={`url(#${body})`}
        />
        <path
          d="M21 57 C34 52 49 60 60 73 C71 60 86 52 99 57 C97 73 82 84 60 84 C38 84 23 73 21 57 Z"
          fill="var(--fox-cream)"
        />

        {mood === 'happy' && (
          <g fill="var(--coral-soft)" opacity="0.4">
            <circle cx="33" cy="64" r="5" />
            <circle cx="87" cy="64" r="5" />
          </g>
        )}
        <Eyes mood={mood} />
        <path d="M54.5 65 Q60 61.5 65.5 65 Q63.5 70.5 60 71.5 Q56.5 70.5 54.5 65 Z" fill={INK} />
        <Mouth mood={mood} />
      </g>

      {mood === 'sleeping' && (
        <g
          fill="none"
          stroke="var(--sky)"
          strokeWidth="3"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path className="fox-zzz" d="M92 24 H102 L92 37 H102" />
          <path className="fox-zzz fox-zzz-2" d="M106 8 H113 L106 17 H113" strokeWidth="2.4" />
        </g>
      )}
    </svg>
  )
}
