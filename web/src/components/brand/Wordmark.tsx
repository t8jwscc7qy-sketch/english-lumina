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

/** Маскот «Вёрст» (пиксельный лисёнок). `scale` — размер клетки в пикселях, ширина = 28 × scale. Декоративный, если не задан title. */
export function Mascot({
  scale = 4,
  className,
  title,
  mood,
}: BrandProps & { title?: string; mood?: FoxMood }) {
  return <Fox scale={scale} mood={mood} className={className} title={title} />
}

/** Столб без лица: для малых размеров (иконка, вкладка). */
export function Post({ scale = 2, className }: BrandProps) {
  return <PixelArtSvg art={post} scale={scale} className={className} />
}
