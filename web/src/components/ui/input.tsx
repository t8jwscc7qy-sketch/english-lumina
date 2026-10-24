import * as React from 'react'
import { cn } from '@/lib/utils'

/** Поле ввода Verst: тонкая контрастная рамка, скругление. Ошибка (aria-invalid) красит рамку в коралл. */
function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'h-11 w-full min-w-0 rounded-lg border border-input bg-card px-4 text-base text-card-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-coral aria-invalid:shadow-[0_0_0_1px_var(--coral)]',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
