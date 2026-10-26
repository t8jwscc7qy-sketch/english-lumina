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

/** Громкость по умолчанию ниже максимума: записи Tatoeba и синтез браузера бывают очень громкими. */
export const DEFAULT_VOLUME = 0.4

/*
 * Голос синтеза. Браузер по умолчанию может взять «игрушечный» голос macOS (Albert, Bad News, Zarvox…) или резкий
 * голос — выбираем спокойный английский голос из известных, иначе первый английский не из игрушечных.
 */
const NOVELTY = /^(Albert|Bad News|Bahh|Bells|Boing|Bubbles|Cellos|Good News|Jester|Organ|Superstar|Trinoids|Whisper|Wobble|Zarvox|Fred|Junior|Ralph|Kathy|Grandma|Grandpa|Rocko|Shelley|Sandy|Flo|Eddy|Reed)\b/
const PREFERRED = ['Samantha', 'Google US English', 'Microsoft Aria', 'Microsoft Jenny', 'Ava', 'Allison', 'Karen', 'Daniel', 'Serena', 'Moira', 'Tessa', 'Google UK English Female']

export function pickVoice(voices: SpeechSynthesisVoice[]): SpeechSynthesisVoice | undefined {
  const english = voices.filter((v) => v.lang.toLowerCase().startsWith('en') && !NOVELTY.test(v.name))
  for (const name of PREFERRED) {
    const v = english.find((x) => x.name.startsWith(name))
    if (v) return v
  }
  return english.find((v) => v.lang === 'en-US' && v.localService) ?? english.find((v) => v.lang === 'en-US') ?? english[0]
}

function englishVoice(): SpeechSynthesisVoice | undefined {
  if (!canSpeak()) return undefined
  if (typeof window.speechSynthesis.getVoices !== 'function') return undefined
  return pickVoice(window.speechSynthesis.getVoices())
}

/** Произнести одну фразу синтезом (тихо, приятным голосом). Прерывает прежнее чтение. */
export function sayText(text: string, rate = 0.9): void {
  stopSpeaking()
  if (!canSpeak()) return
  const u = new SpeechSynthesisUtterance(text)
  u.lang = 'en-US'
  u.rate = rate
  u.volume = DEFAULT_VOLUME
  const v = englishVoice()
  if (v) u.voice = v
  window.speechSynthesis.speak(u)
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
  u.volume = DEFAULT_VOLUME
  const voice = englishVoice()
  if (voice) u.voice = voice
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
      a.volume = DEFAULT_VOLUME
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
