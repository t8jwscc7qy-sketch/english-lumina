import { Check, Plus } from 'lucide-react'
import { useRef } from 'react'
import { Button } from '@/components/ui/button'
import { useAddCard } from '@/features/cards/cards'

/**
 * Кнопка в результате «выбора формы». Сделана по образцу «В карточки» на странице «Слова»:
 * слово без примеров сервер отклоняет (409 no_examples), его текст показываем под кнопкой.
 * Нажатие мышью не забирает фокус у карточки упражнения, чтобы Пробел и Enter по-прежнему вели к следующему.
 */
export function AddToCards({ word }: { word: { lemma: string; pos: string; inCards: boolean } }) {
  const add = useAddCard()
  const busy = useRef(false)

  if (word.inCards || add.isSuccess) {
    return (
      <Button variant="ghost" size="sm" disabled className="-ml-3">
        <Check className="size-4" aria-hidden />В карточках
      </Button>
    )
  }
  return (
    <div className="flex flex-col items-start gap-1">
      <Button
        variant="secondary"
        size="sm"
        disabled={add.isPending}
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => {
          if (busy.current) return // двойной клик: пока идёт запрос, второго нет
          busy.current = true
          add.mutate({ lemma: word.lemma, pos: word.pos }, { onSettled: () => (busy.current = false) })
        }}
      >
        {add.isPending ? (
          'Добавляем…'
        ) : (
          <>
            <Plus className="size-4" aria-hidden />Добавить в карточки
          </>
        )}
      </Button>
      {add.isError && (
        <p role="alert" className="text-sm text-coral-text">
          {add.error.message}
        </p>
      )}
    </div>
  )
}
