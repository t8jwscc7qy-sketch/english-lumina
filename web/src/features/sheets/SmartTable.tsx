import { useMemo, useState } from 'react'
import type { SheetBlock, SheetRow } from '@/api/client'
import { RichText } from './richText'

type Filter = 'all' | 'weak' | 'unseen'

/** Текст без разметки **выделения**: сортировка и поиск не должны зависеть от звёздочек. */
const plain = (t: string) => t.replace(/\*\*/g, '')

const STATUS: Record<string, { icon: string; label: string }> = {
  unseen: { icon: '○', label: 'не встречал' },
  introduced: { icon: '◐', label: 'показано' },
  learning: { icon: '◑', label: 'учится' },
  learned: { icon: '●', label: 'изучено' },
  known_at_start: { icon: '✓', label: 'знаю' },
  hard: { icon: '!', label: 'трудно' },
  weak: { icon: '!', label: 'слабая' },
  ok: { icon: '✓', label: 'в порядке' },
}

function StatusBadge({ status, topic }: { status: string; topic: boolean }) {
  const s = topic && status === 'unseen' ? { icon: '○', label: 'не тренировал' } : (STATUS[status] ?? STATUS.unseen)
  const alert = status === 'hard' || status === 'weak'
  return (
    <span className={alert ? 'font-semibold text-coral-text' : 'text-muted-foreground'}>
      <span aria-hidden="true">{s.icon} </span>
      {s.label}
    </span>
  )
}

/** Строка из «моих слабых»: трудное слово или слабая тема. */
function isWeak(r: SheetRow) {
  return r.status === 'hard' || r.status === 'weak'
}

/**
 * Таблица шпаргалки. Привязанная к данным (слова или темы) показывает статус строки значком и словом, умеет
 * фильтры «Мои слабые» и «Ещё не учил». Поиск и сортировка по столбцу работают на клиенте, без запросов.
 */
export function SmartTable({ block }: { block: SheetBlock }) {
  const columns = block.columns ?? []
  const rows = useMemo(() => block.rows ?? [], [block.rows])
  const bound = block.bind === 'word' || block.bind === 'topic'
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [sort, setSort] = useState<{ col: number; dir: 'asc' | 'desc' } | null>(null)

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    let out = rows.filter((r) => (filter === 'weak' ? isWeak(r) : filter === 'unseen' ? r.status === 'unseen' : true))
    if (q) out = out.filter((r) => r.cells.some((c) => plain(c).toLowerCase().includes(q)))
    if (sort) {
      const k = sort.col
      out = [...out].sort((a, b) => plain(a.cells[k]).localeCompare(plain(b.cells[k])) * (sort.dir === 'asc' ? 1 : -1))
    }
    return out
  }, [rows, query, filter, sort])

  const label = block.title ?? 'Таблица'
  const weakTotal = rows.filter(isWeak).length

  return (
    <section aria-label={label} className="flex flex-col gap-3">
      {block.title && <h2 className="text-title">{block.title}</h2>}

      {(bound || rows.length > 8) && (
        <div className="no-print flex flex-wrap items-center gap-3">
          {rows.length > 8 && (
            <label className="flex items-center gap-2 text-sm">
              <span className="sr-only">Поиск по таблице</span>
              <input
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Поиск"
                className="h-10 w-48 border border-input bg-card px-3"
              />
            </label>
          )}
          {bound && (
            <div role="group" aria-label="Фильтр строк" className="flex flex-wrap gap-2">
              {(
                [
                  ['all', 'Все'],
                  ['weak', `Мои слабые (${weakTotal})`],
                  ['unseen', 'Ещё не учил'],
                ] as const
              ).map(([f, text]) => (
                <button
                  key={f}
                  type="button"
                  aria-pressed={filter === f}
                  onClick={() => setFilter(f)}
                  className={`pixel-frame min-h-10 px-3 text-sm font-semibold ${filter === f ? 'bg-primary text-primary-foreground' : 'bg-card'}`}
                >
                  {text}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Широкая таблица прокручивается по горизонтали; область доступна с клавиатуры */}
      <div role="region" aria-label={`${label}: прокрутка`} tabIndex={0} className="overflow-x-auto border border-input">
        <table className="w-full min-w-max border-collapse text-left">
          <thead>
            <tr className="bg-muted">
              {columns.map((c, i) => (
                <th
                  key={i}
                  scope="col"
                  aria-sort={sort?.col === i ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
                  className="px-3 py-2 font-semibold"
                >
                  <button
                    type="button"
                    onClick={() => setSort(sort?.col === i && sort.dir === 'asc' ? { col: i, dir: 'desc' } : { col: i, dir: 'asc' })}
                    className="font-semibold"
                    aria-label={`Сортировать по столбцу «${c.title}»`}
                  >
                    {c.title}
                    {sort?.col === i && <span aria-hidden="true">{sort.dir === 'asc' ? ' ▲' : ' ▼'}</span>}
                  </button>
                </th>
              ))}
              {bound && (
                <th scope="col" className="px-3 py-2 font-semibold">
                  {block.bind === 'topic' ? 'Твой уровень' : 'Статус'}
                </th>
              )}
            </tr>
          </thead>
          <tbody>
            {shown.map((r, ri) => (
              <tr key={r.key ?? ri} className="border-t border-input">
                {r.cells.map((cell, ci) => (
                  <td key={ci} className="px-3 py-2 align-top">
                    <RichText text={cell} />
                    {ci === 0 && r.needsCheck && <span className="ml-2 text-xs font-semibold text-coral-text">проверить</span>}
                  </td>
                ))}
                {bound && (
                  <td className="px-3 py-2 align-top whitespace-nowrap">
                    <StatusBadge status={r.status ?? 'unseen'} topic={block.bind === 'topic'} />
                    {r.rating != null && <span className="ml-2 text-xs text-muted-foreground tabular-nums">{Math.round(r.rating)}</span>}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {shown.length > 0 && shown.length < rows.length && (
        <p className="hidden text-sm print:block">{`Показаны не все строки: ${shown.length} из ${rows.length} (включён фильтр или поиск).`}</p>
      )}

      {shown.length === 0 && (
        <p role="status" className="text-muted-foreground">
          {rows.length === 0
            ? block.bind === 'mistake'
              ? 'Открытых ошибок нет. Они появятся здесь, когда ответишь неверно.'
              : 'Строк нет.'
            : 'Ничего не нашлось: поменяй поиск или фильтр.'}
        </p>
      )}
    </section>
  )
}
