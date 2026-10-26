import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('чтение и слушание', () => {
  test('читать: подсказка к слову, карточка, оценка и следующий текст', async ({ page }) => {
    const rates: unknown[] = []
    const knows: unknown[] = []
    const modes: string[] = []
    await mockApi(page, { loggedIn: true, read: { rates, knows, modes } })
    await page.goto('/read')
    await expect(page.getByText('Понимание этого текста: 86% (цель 85%)')).toBeVisible()
    await page.getByRole('button', { name: 'dog' }).first().click()
    await expect(page.getByRole('region', { name: 'Слово dog' })).toContainText('собака')
    await page.getByRole('button', { name: 'Знаю' }).click()
    await expect(page.getByRole('button', { name: 'Отмечено как известное' })).toBeVisible()
    expect(knows[0]).toEqual({ lexemeId: 11 })
    await page.screenshot({ path: 'test-results/screens/read-1440.png' })
    await page.getByRole('button', { name: 'Нормально' }).click()
    await expect(page.getByText('Принято. Цель понимания теперь 83%.')).toBeVisible()
    expect(rates[0]).toEqual({ textId: 7, understood: 'ok', mode: 'read' })
    await page.getByRole('button', { name: 'Следующий текст' }).click()
    await expect(page.getByRole('button', { name: 'Нормально' })).toBeVisible()
    expect(modes.length).toBe(2)
  })

  test('слушать: текст скрыт, пока не нажать «Показать текст»; режим запоминается в адресе', async ({ page }) => {
    const modes: string[] = []
    await mockApi(page, { loggedIn: true, read: { modes } })
    await page.goto('/read')
    await page.getByRole('button', { name: 'Слушать', exact: true }).click()
    await expect(page).toHaveURL(/mode=listen/)
    await expect(page.getByRole('button', { name: 'Показать текст' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'dog' })).toHaveCount(0)
    await page.getByRole('button', { name: 'Показать текст' }).click()
    await expect(page.getByRole('button', { name: 'dog' }).first()).toBeVisible()
    expect(modes).toContain('listen')
  })

  test('нет текста: совет начать со знакомства', async ({ page }) => {
    await mockApi(page, { loggedIn: true, read: { noText: true } })
    await page.goto('/read')
    await expect(page.getByRole('heading', { name: 'Пока не из чего собрать текст' })).toBeVisible()
    await page.getByRole('link', { name: 'Знакомство' }).click()
    await expect(page).toHaveURL(/\/learn$/)
  })

  test('пункт меню «Читать» и ссылка в карточке «Знакомство»', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: { campWords: 12 } })
    await page.goto('/')
    await page.getByRole('link', { name: 'Читать', exact: true }).first().click()
    await expect(page).toHaveURL(/\/read$/)
  })

  test('клавиатура: слово нажимается Пробелом', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/read')
    await page.getByRole('button', { name: 'run' }).focus()
    await page.keyboard.press('Space')
    await expect(page.getByRole('region', { name: 'Слово run' })).toBeVisible()
  })

  for (const size of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    test(`вид ${size.width}px без горизонтальной прокрутки`, async ({ page }) => {
      await page.setViewportSize(size)
      await mockApi(page, { loggedIn: true })
      await page.goto('/read')
      await expect(page.getByText(/Понимание этого текста/)).toBeVisible()
      await page.getByRole('button', { name: 'dog' }).first().click()
      expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false)
      await page.screenshot({ path: `test-results/screens/read-${size.width}.png` })
    })
  }
})
