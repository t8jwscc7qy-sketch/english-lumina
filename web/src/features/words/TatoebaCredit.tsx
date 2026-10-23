type Sentence = { tatoebaId: number; author: string }

const tatoebaUrl = (id: number) => `https://tatoeba.org/en/sentences/show/${id}`

function TatoebaLink({ id }: { id: number }) {
  return (
    <a href={tatoebaUrl(id)} target="_blank" rel="noreferrer noopener" className="underline underline-offset-4 hover:no-underline">
      #{id}
    </a>
  )
}

/** Атрибуция по лицензии CC BY: номер и автор обеих сторон со ссылками на tatoeba.org. */
export function TatoebaCredit({ en, ru }: { en: Sentence; ru: Sentence }) {
  return (
    <p className="text-xs text-muted-foreground">
      Tatoeba: <TatoebaLink id={en.tatoebaId} /> {en.author} · <TatoebaLink id={ru.tatoebaId} /> {ru.author}
    </p>
  )
}
