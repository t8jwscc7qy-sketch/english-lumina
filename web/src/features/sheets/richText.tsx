import { splitBold } from './bold'

/** Текст с выделением: всё выводится как текст, разметки HTML нет. */
export function RichText({ text }: { text: string }) {
  return (
    <>
      {splitBold(text).map((p, i) => (p.bold ? <strong key={i}>{p.text}</strong> : <span key={i}>{p.text}</span>))}
    </>
  )
}
