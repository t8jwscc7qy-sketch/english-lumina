import { Button } from '@/components/ui/button'
import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { icons } from '@/components/pixel/icons'
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
      <PixelArtSvg art={toDark ? icons.moon : icons.today} scale={3} />
    </Button>
  )
}
