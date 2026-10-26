import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('шпаргалки', () => {
  test('список: поиск, теги, закрепление и блок на «Сегодня»', async ({ page }) => {
    const favorites: unknown[] = []
    await mockApi(page, { loggedIn: true, sheets: { favorites } })
    await page.goto('/sheets')
    await expect(page.getByRole('heading', { name: 'Шпаргалки', level: 1 })).toBeVisible()
    await page.getByRole('searchbox', { name: 'Поиск шпаргалки' }).fill('артикл')
    await expect(page.getByRole('link', { name: /Артикли/ })).toBeVisible()
    await expect(page.getByRole('link', { name: /Глагол to be/ })).toHaveCount(0)
    await page.getByRole('searchbox', { name: 'Поиск шпаргалки' }).fill('')
    await page.getByRole('link', { name: /Неправильные глаголы/ }).click()
    await expect(page).toHaveURL(/\/sheets\/irregular-verbs$/)
    await page.getByRole('button', { name: '☆ Закрепить' }).click()
    await expect(page.getByRole('button', { name: '★ Закреплено' })).toBeVisible()
    expect(favorites[0]).toEqual({ id: 'irregular-verbs', favorite: true })
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Закреплённое' })).toBeVisible()
  })

  test('умная таблица: фильтр «Мои слабые», сортировка, поиск', async ({ page }) => {
    await mockApi(page, { loggedIn: true, sheets: { hard: 2 } })
    await page.goto('/sheets/irregular-verbs')
    await expect(page.getByText('Черновик: проверь, прежде чем заучивать.')).toBeVisible()
    await expect(page.getByText('трудно')).toHaveCount(2)
    await page.getByRole('button', { name: 'Мои слабые (2)' }).click()
    await expect(page.getByRole('row')).toHaveCount(3) // шапка и две строки
    await page.getByRole('button', { name: 'Все' }).click()
    await page.getByRole('button', { name: 'Сортировать по столбцу «Глагол»' }).click()
    await expect(page.getByRole('row').nth(1)).toContainText('come')
    await page.getByRole('searchbox', { name: 'Поиск по таблице' }).fill('write')
    await expect(page.getByRole('row')).toHaveCount(2)
  })

  test('«Я проверил» меняет плашку', async ({ page }) => {
    const reviews: unknown[] = []
    await mockApi(page, { loggedIn: true, sheets: { reviews } })
    await page.goto('/sheets/to-be')
    await page.getByRole('button', { name: 'Я проверил' }).click()
    await expect(page.getByText('✓ Проверено')).toBeVisible()
    expect(reviews[0]).toEqual({ id: 'to-be' })
  })

  test('печать: меню и кнопки скрыты, таблица на всю ширину', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/sheets/irregular-verbs')
    await expect(page.getByRole('heading', { name: 'Неправильные глаголы', level: 1 })).toBeVisible()
    await page.emulateMedia({ media: 'print' })
    await expect(page.getByRole('navigation', { name: 'Разделы' })).toBeHidden()
    await expect(page.getByRole('button', { name: 'Печать' })).toBeHidden()
    await expect(page.getByRole('table')).toBeVisible()
  })

  test('клавиатура: фильтр переключается Пробелом', async ({ page }) => {
    await mockApi(page, { loggedIn: true, sheets: { hard: 1 } })
    await page.goto('/sheets/irregular-verbs')
    await page.getByRole('button', { name: 'Мои слабые (1)' }).focus()
    await page.keyboard.press('Space')
    await expect(page.getByRole('button', { name: 'Мои слабые (1)' })).toHaveAttribute('aria-pressed', 'true')
  })

  for (const size of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    test(`вид ${size.width}px: страница не прокручивается вбок, таблица прокручивается`, async ({ page }) => {
      await page.setViewportSize(size)
      await mockApi(page, { loggedIn: true, sheets: { hard: 2 } })
      await page.goto('/sheets/irregular-verbs')
      await expect(page.getByRole('table')).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false)
      await page.screenshot({ path: `test-results/screens/sheet-${size.width}.png` })
      await page.goto('/sheets')
      await expect(page.getByRole('heading', { name: 'Шпаргалки', level: 1 })).toBeVisible()
      await page.screenshot({ path: `test-results/screens/sheets-${size.width}.png` })
    })
  }
})
