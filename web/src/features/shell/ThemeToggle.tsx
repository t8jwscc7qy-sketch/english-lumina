import { Contrast } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTheme } from '@/lib/theme'

/** Переключатель светлой и тёмной темы. Название кнопки говорит, что включится после нажатия. */
export function ThemeToggle({ className }: { className?: string }) {
  const { resolved, setTheme } = useTheme()
  const toDark = resolved === 'light'
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className={className}
      aria-label={toDark ? 'Тёмная тема' : 'Светлая тема'}
      title={toDark ? 'Включить тёмную тему' : 'Включить светлую тему'}
      onClick={() => setTheme(toDark ? 'dark' : 'light')}
    >
      <Contrast className="size-5" strokeWidth={1.75} aria-hidden />
    </Button>
  )
}
