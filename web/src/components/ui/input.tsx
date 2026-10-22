import * as React from 'react'
import { cn } from '@/lib/utils'

/** Поле ввода Verst: 4px контур, прямые углы. Ошибка (aria-invalid) красит контур в коралл. */
function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'h-12 w-full min-w-0 border-4 border-line bg-card px-4 text-base text-card-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-coral',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
