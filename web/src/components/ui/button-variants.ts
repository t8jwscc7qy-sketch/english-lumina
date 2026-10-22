import { cva } from 'class-variance-authority'

/*
 * Варианты кнопки Verst: 4px контур, жёсткая тень; при нажатии «вдавливается» на 4px.
 * Переходов нет — состояния меняются мгновенно, как кадры в пиксельной игре.
 * Главное действие (default) — жёлтое: этот цвет зарезервирован под опыт и главное действие.
 */
export const buttonVariants = cva(
  'inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 border-4 border-line font-pixel whitespace-nowrap select-none disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground shadow-pixel hover:brightness-105 active:translate-x-1 active:translate-y-1 active:shadow-pixel-sm',
        secondary:
          'bg-card text-card-foreground shadow-pixel hover:bg-muted active:translate-x-1 active:translate-y-1 active:shadow-pixel-sm',
        accent:
          'bg-accent text-accent-foreground shadow-pixel hover:brightness-105 active:translate-x-1 active:translate-y-1 active:shadow-pixel-sm',
        destructive:
          'bg-destructive text-ink shadow-pixel hover:brightness-105 active:translate-x-1 active:translate-y-1 active:shadow-pixel-sm',
        ghost: 'border-transparent bg-transparent text-foreground hover:border-line hover:bg-muted',
        link: 'border-transparent bg-transparent px-0 text-foreground underline underline-offset-4 hover:no-underline',
      },
      size: {
        default: 'h-12 px-6 text-base',
        sm: 'h-10 px-4 text-sm',
        lg: 'h-14 px-8 text-xl',
        icon: 'size-12',
        'icon-sm': 'size-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)
