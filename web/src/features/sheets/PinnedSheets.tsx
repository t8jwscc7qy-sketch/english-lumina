import { Link } from 'react-router'
import { pinned, useSheets } from './sheets'

/** «Закреплённое» на «Сегодня»: до трёх шпаргалок, недавно открытые первыми. Нет закреплённых или ошибка — блока нет. */
export function PinnedSheets() {
  const sheets = useSheets()
  const list = pinned(sheets.data ?? []).slice(0, 3)
  if (list.length === 0) return null
  return (
    <section aria-labelledby="pinned-title" className="surface flex flex-col gap-3 p-6">
      <h2 id="pinned-title" className="text-title">Закреплённое</h2>
      <ul className="flex flex-wrap gap-x-6 gap-y-2">
        {list.map((s) => (
          <li key={s.id}>
            <Link to={`/sheets/${s.id}`} className="font-semibold underline underline-offset-4">
              {s.title}
            </Link>
            {s.weakCount > 0 && <span className="ml-2 text-sm text-coral-text">{`слабых: ${s.weakCount}`}</span>}
          </li>
        ))}
      </ul>
    </section>
  )
}
