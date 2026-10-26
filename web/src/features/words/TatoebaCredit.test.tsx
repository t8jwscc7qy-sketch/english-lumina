import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { TatoebaCredit } from './TatoebaCredit'

describe('TatoebaCredit', () => {
  it('фраза Tatoeba: номера и авторы обеих сторон со ссылками', () => {
    render(<TatoebaCredit en={{ tatoebaId: 11, author: 'tom' }} ru={{ tatoebaId: 12, author: 'ivan' }} />)
    expect(screen.getByRole('link', { name: '#11' })).toHaveAttribute('href', 'https://tatoeba.org/en/sentences/show/11')
    expect(screen.getByRole('link', { name: '#12' })).toBeInTheDocument()
  })
  it('фраза из авторского текста: без ссылок на Tatoeba', () => {
    render(<TatoebaCredit en={{ tatoebaId: 0, author: 'Verst' }} ru={{ tatoebaId: 0, author: 'Verst' }} />)
    expect(screen.getByText('Фраза из авторского текста Verst')).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
