import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useDebounced } from './useDebounced'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('useDebounced', () => {
  it('сразу отдаёт начальное значение', () => {
    const { result } = renderHook(() => useDebounced('a', 250))
    expect(result.current).toBe('a')
  })

  it('меняет значение только после паузы, а серия правок даёт одно обновление', () => {
    const { result, rerender } = renderHook(({ v }) => useDebounced(v, 250), { initialProps: { v: 'a' } })
    rerender({ v: 'ab' })
    act(() => vi.advanceTimersByTime(200))
    rerender({ v: 'abc' })
    act(() => vi.advanceTimersByTime(200))
    expect(result.current).toBe('a')
    act(() => vi.advanceTimersByTime(60))
    expect(result.current).toBe('abc')
  })
})
