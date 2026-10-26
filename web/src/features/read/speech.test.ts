import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { canPlay, speak, stopSpeaking, pickVoice } from './speech'

type Handler = (() => void) | null

class FakeAudio {
  static instances: FakeAudio[] = []
  static failPlay = false
  url: string
  playbackRate = 1
  onplay: Handler = null
  onended: Handler = null
  onerror: Handler = null
  paused = false
  constructor(url: string) {
    this.url = url
    FakeAudio.instances.push(this)
  }
  play() {
    if (FakeAudio.failPlay) return Promise.reject(new Error('blocked'))
    queueMicrotask(() => {
      this.onplay?.()
      this.onended?.()
    })
    return Promise.resolve()
  }
  pause() {
    this.paused = true
  }
}

function stubSynth(spoken: string[]) {
  class Utt {
    text: string
    lang = ''
    rate = 1
    onstart: Handler = null
    onend: Handler = null
    onerror: Handler = null
    onboundary: ((e: { charIndex: number }) => void) | null = null
    constructor(t: string) {
      this.text = t
    }
  }
  vi.stubGlobal('SpeechSynthesisUtterance', Utt)
  vi.stubGlobal('speechSynthesis', {
    cancel: () => {},
    speak: (u: Utt) => {
      spoken.push(u.text)
      queueMicrotask(() => {
        u.onstart?.()
        u.onend?.()
      })
    },
  })
}

beforeEach(() => {
  FakeAudio.instances = []
  FakeAudio.failPlay = false
  vi.stubGlobal('Audio', FakeAudio)
})
afterEach(() => {
  stopSpeaking()
  vi.unstubAllGlobals()
})

const flush = () => new Promise((r) => setTimeout(r, 10))

describe('speech', () => {
  it('предложение с записью играет файл, а не синтез; следующее без записи — синтез', async () => {
    const spoken: string[] = []
    stubSynth(spoken)
    const started: number[] = []
    let ended = false
    speak([{ text: 'One.', audioUrl: 'https://x/1' }, { text: 'Two.' }], 0.7, { onSentence: (i) => started.push(i), onEnd: () => (ended = true) })
    await flush()
    expect(FakeAudio.instances.map((a) => a.url)).toEqual(['https://x/1'])
    expect(FakeAudio.instances[0].playbackRate).toBe(0.7)
    expect(spoken).toEqual(['Two.'])
    expect(started).toEqual([0, 1])
    expect(ended).toBe(true)
  })

  it('запись не проигралась (ошибка) — то же предложение читает синтез', async () => {
    const spoken: string[] = []
    stubSynth(spoken)
    FakeAudio.failPlay = true
    speak([{ text: 'Broken audio.', audioUrl: 'https://x/bad' }], 1)
    await flush()
    expect(spoken).toEqual(['Broken audio.'])
  })

  it('остановка прерывает запись и не даёт читать дальше', async () => {
    const spoken: string[] = []
    stubSynth(spoken)
    FakeAudio.prototype.play = function () {
      return new Promise(() => {}) // запись «идёт» бесконечно
    }
    speak([{ text: 'A.', audioUrl: 'https://x/a' }, { text: 'B.' }], 1)
    stopSpeaking()
    await flush()
    expect(FakeAudio.instances[0].paused).toBe(true)
    expect(spoken).toEqual([])
  })

  it('canPlay: нужен либо синтез, либо запись', () => {
    vi.stubGlobal('speechSynthesis', undefined)
    expect(canPlay([{ text: 'x' }])).toBe(false)
    expect(canPlay([{ text: 'x', audioUrl: 'https://x/1' }])).toBe(true)
    stubSynth([])
    expect(canPlay([{ text: 'x' }])).toBe(true)
  })
})

describe('pickVoice', () => {
  const v = (name: string, lang = 'en-US', localService = true) => ({ name, lang, localService }) as SpeechSynthesisVoice
  it('берёт спокойный известный голос и не берёт игрушечные', () => {
    expect(pickVoice([v('Albert'), v('Zarvox'), v('Samantha'), v('Milena', 'ru-RU')])?.name).toBe('Samantha')
    expect(pickVoice([v('Bad News'), v('Some Voice')])?.name).toBe('Some Voice')
    expect(pickVoice([v('Milena', 'ru-RU')])).toBeUndefined()
  })
})
