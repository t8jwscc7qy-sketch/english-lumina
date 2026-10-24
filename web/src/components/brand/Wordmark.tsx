import { PixelArtSvg } from '@/components/pixel/PixelArtSvg'
import { mascot, post, wordmarkGround } from './logo-data'

interface BrandProps {
  scale?: number
  className?: string
}

/** Слово VERST на земле: основной знак в навигации. */
export function Wordmark({ scale = 3, className }: BrandProps) {
  return <PixelArtSvg art={wordmarkGround} scale={scale} title="Verst" className={className} />
}

/** Маскот «Верста»: полосатый столбик-веха с лицом. Декоративный, если не задан title. */
export function Mascot({ scale = 4, className, title }: BrandProps & { title?: string }) {
  return <PixelArtSvg art={mascot} scale={scale} title={title} className={className} />
}

/** Столб без лица: для малых размеров (иконка, вкладка). */
export function Post({ scale = 2, className }: BrandProps) {
  return <PixelArtSvg art={post} scale={scale} className={className} />
}
