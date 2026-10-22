import { Toaster as Sonner, type ToasterProps } from 'sonner'

/** Уведомления в стиле Verst: рамка 4px и жёсткая тень; цвета берутся из токенов, тема переключается классом. */
const Toaster = (props: ToasterProps) => (
  <Sonner
    position="bottom-right"
    toastOptions={{
      unstyled: true,
      classNames: {
        toast:
          'pixel-box flex w-[22rem] max-w-[calc(100vw-2rem)] items-start gap-3 p-4 font-sans text-base',
        title: 'font-semibold',
        description: 'text-muted-foreground text-sm',
        error: '!border-coral',
        success: '!border-grass',
      },
    }}
    {...props}
  />
)

export { Toaster }
