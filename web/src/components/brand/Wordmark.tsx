import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { post, wordmarkGround } from './logo-data'
import { Fox, type FoxMood } from './Fox'

interface BrandProps {
  scale?: number
  className?: string
}

/** Слово VERST на земле: основной знак в навигации. */
export function Wordmark({ scale = 3, className }: BrandProps) {
  return <PixelArtSvg art={wordmarkGround} scale={scale} title="Verst" className={className} />
}

/** Маскот «Вёрст» (лисёнок-путник). `scale` сохранён от пиксельного маскота: ширина = 12 × scale px. Декоративный, если не задан title. */
export function Mascot({
  scale = 4,
  className,
  title,
  mood,
}: BrandProps & { title?: string; mood?: FoxMood }) {
  return <Fox size={12 * scale} mood={mood} className={className} title={title} />
}

/** Столб без лица: для малых размеров (иконка, вкладка). */
export function Post({ scale = 2, className }: BrandProps) {
  return <PixelArtSvg art={post} scale={scale} className={className} />
}
