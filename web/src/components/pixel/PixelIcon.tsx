import { PixelArtSvg } from './PixelArtSvg'
import { iconArt, type IconName } from './icons'

/** Пиксельная иконка раздела 12×12 клеток цвета текста. Декоративная: подпись всегда рядом. */
export function PixelIcon({ name, scale = 2, className }: { name: IconName; scale?: number; className?: string }) {
  return <PixelArtSvg art={iconArt(name)} scale={scale} className={className} />
}
