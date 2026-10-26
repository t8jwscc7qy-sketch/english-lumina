/** Тонкая обёртка над синтезом речи браузера. Без него (нет `speechSynthesis`) звук недоступен, а чтение работает. */

export function canSpeak(): boolean {
  return typeof window !== 'undefined' && 'speechSynthesis' in window && typeof SpeechSynthesisUtterance !== 'undefined'
}

export type SpeakHandlers = {
  /** Началось предложение с номером i. */
  onSentence?: (i: number) => void
  /** Читается слово с номером w (по порядку слов) предложения i; приходит не во всех браузерах. */
  onWord?: (i: number, charIndex: number) => void
  onEnd?: () => void
}

let session = 0

// Chrome может собрать мусором высказывание без ссылки, и тогда `onend` не придёт: держим ссылки, пока идёт чтение.
const live = new Set<SpeechSynthesisUtterance>()

/** Читает предложения подряд; повторный вызов или `stopSpeaking` прерывает прежнее чтение. */
export function speak(sentences: string[], rate: number, handlers: SpeakHandlers = {}, from = 0): void {
  if (!canSpeak()) return
  window.speechSynthesis.cancel()
  const mine = ++session
  const next = (i: number) => {
    if (mine !== session) return
    if (i >= sentences.length) {
      handlers.onEnd?.()
      return
    }
    const u = new SpeechSynthesisUtterance(sentences[i])
    u.lang = 'en-US'
    u.rate = rate
    u.onstart = () => handlers.onSentence?.(i)
    u.onboundary = (e) => handlers.onWord?.(i, e.charIndex)
    live.add(u)
    u.onend = () => {
      live.delete(u)
      next(i + 1)
    }
    u.onerror = () => {
      live.delete(u)
      if (mine === session) handlers.onEnd?.()
    }
    window.speechSynthesis.speak(u)
  }
  next(from)
}

export function stopSpeaking(): void {
  session++
  live.clear()
  if (canSpeak()) window.speechSynthesis.cancel()
}
