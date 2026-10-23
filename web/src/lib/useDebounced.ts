import { useEffect, useState } from 'react'

/** Значение, которое обновляется только после паузы в `ms` миллисекунд: серия правок даёт одно обновление. */
export function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return debounced
}
