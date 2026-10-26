import { screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { errorResponse, jsonResponse, renderAt } from '@/test/render'
import { pathFixture } from './fixtures'
import { PathPage } from './PathPage'

afterEach(() => vi.unstubAllGlobals())

const render = () => renderAt('/path', [{ path: '/path', element: <PathPage /> }])

describe('PathPage', () => {
  it('крупно показывает следующий урок и дорогу уроков с состояниями', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, pathFixture)))
    render()
    expect(await screen.findByText('Урок 2: Как дела?')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Продолжить' })).toHaveAttribute('href', '/lesson')
    expect(screen.getByRole('heading', { name: 'Блок 1. Знакомство' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Урок 1: Привет, пройден, пройти ещё раз' })).toHaveAttribute('href', '/lesson?id=u1-l1')
    expect(screen.getByRole('link', { name: 'Урок 2: Как дела?, начать' })).toHaveAttribute('href', '/lesson')
    expect(screen.queryByRole('link', { name: /Кто это/ })).toBeNull()
    expect(screen.getByLabelText('Урок 3: Кто это?, закрыт')).toBeInTheDocument()
    expect(screen.getByText('1 из 3')).toBeInTheDocument()
  })

  it('курс пройден — спокойное сообщение вместо кнопки', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(200, { ...pathFixture, next: undefined })))
    render()
    expect(await screen.findByText('Все уроки пройдены')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Продолжить' })).toBeNull()
  })

  it('ошибка загрузки — сообщение и «Повторить»', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => errorResponse(500, 'internal', 'Сервер упал')))
    render()
    expect(await screen.findByRole('alert')).toHaveTextContent('Сервер упал')
    expect(screen.getByRole('button', { name: 'Повторить' })).toBeInTheDocument()
  })
})
