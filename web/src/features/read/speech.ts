/**
 * Озвучка: живая запись (если у предложения есть — файл Tatoeba) или синтез речи браузера. Без обоих звук недоступен,
 * а чтение работает. Запасной вариант срабатывает и когда запись не загрузилась или браузер не дал её проиграть.
 */

export function canSpeak(): boolean {
  return typeof window !== 'undefined' && 'speechSynthesis' in window && typeof SpeechSynthesisUtterance !== 'undefined'
}

/** Предложение для озвучки: текст (для синтеза) и необязательный адрес живой записи. */
export type SpeakItem = { text: string; audioUrl?: string }

/** Можно ли озвучить хотя бы одно предложение из списка. */
export function canPlay(items: SpeakItem[]): boolean {
  return canSpeak() || (typeof Audio !== 'undefined' && items.some((i) => i.audioUrl))
}

export type SpeakHandlers = {
  /** Началось предложение с номером i (в переданном списке). */
  onSentence?: (i: number) => void
  /** Читается слово предложения i (синтез речи, если браузер даёт `boundary`); для живой записи не приходит. */
  onWord?: (i: number, charIndex: number) => void
  onEnd?: () => void
}

let session = 0
let current: HTMLAudioElement | null = null

// Chrome может собрать мусором высказывание без ссылки, и тогда `onend` не придёт: держим ссылки, пока идёт чтение.
const live = new Set<SpeechSynthesisUtterance>()

function synth(item: SpeakItem, i: number, rate: number, handlers: SpeakHandlers, done: () => void) {
  if (!canSpeak()) {
    done()
    return
  }
  const u = new SpeechSynthesisUtterance(item.text)
  u.lang = 'en-US'
  u.rate = rate
  u.onstart = () => handlers.onSentence?.(i)
  u.onboundary = (e) => handlers.onWord?.(i, e.charIndex)
  live.add(u)
  u.onend = () => {
    live.delete(u)
    done()
  }
  u.onerror = () => {
    live.delete(u)
    done()
  }
  window.speechSynthesis.speak(u)
}

/** Читает предложения подряд; повторный вызов или `stopSpeaking` прерывает прежнее чтение. */
export function speak(items: SpeakItem[], rate: number, handlers: SpeakHandlers = {}, from = 0): void {
  stopSpeaking()
  const mine = ++session
  const next = (i: number) => {
    if (mine !== session) return
    if (i >= items.length) {
      handlers.onEnd?.()
      return
    }
    const item = items[i]
    const fallback = () => {
      if (mine === session) synth(item, i, rate, handlers, () => next(i + 1))
    }
    if (item.audioUrl && typeof Audio !== 'undefined') {
      const a = new Audio(item.audioUrl)
      a.playbackRate = rate
      current = a
      a.onplay = () => handlers.onSentence?.(i)
      a.onended = () => next(i + 1)
      a.onerror = fallback
      a.play().catch(fallback)
      return
    }
    synth(item, i, rate, handlers, () => next(i + 1))
  }
  next(from)
}

export function stopSpeaking(): void {
  session++
  live.clear()
  if (current) {
    current.onended = null
    current.onerror = null
    current.pause()
    current = null
  }
  if (canSpeak()) window.speechSynthesis.cancel()
}
