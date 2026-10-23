import { cva } from 'class-variance-authority'

/*
 * Варианты кнопки Verst: спокойные, со скруглением и лёгкой тенью.
 * Главное действие (default) — жёлтое: этот цвет зарезервирован под опыт и главное действие.
 * Недоступная кнопка — приглушённая заливка без тени: читается как «пока нельзя».
 */
export const buttonVariants = cva(
  'inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 rounded-lg font-sans font-semibold whitespace-nowrap select-none transition-[background-color,filter,translate] duration-150 disabled:pointer-events-none disabled:bg-muted disabled:text-muted-foreground disabled:shadow-none [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground shadow-button hover:brightness-95 active:translate-y-px',
        secondary:
          'border border-input bg-card text-card-foreground hover:bg-muted active:translate-y-px',
        accent:
          'bg-accent text-accent-foreground shadow-button hover:brightness-95 active:translate-y-px',
        destructive:
          'bg-coral-soft text-ink shadow-button hover:brightness-95 active:translate-y-px',
        ghost: 'bg-transparent text-foreground hover:bg-muted disabled:bg-transparent',
        link: 'bg-transparent px-0 text-foreground underline underline-offset-4 hover:no-underline disabled:bg-transparent',
      },
      size: {
        default: 'h-11 px-5 text-base',
        sm: 'h-9 px-3 text-sm',
        lg: 'min-h-12 px-6 py-2.5 text-base whitespace-normal',
        icon: 'size-11',
        'icon-sm': 'size-9',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)
