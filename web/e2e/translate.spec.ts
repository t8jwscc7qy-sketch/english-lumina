import { expect, test, type Page } from '@playwright/test'
import { mockApi, TRANSLATE_EXERCISE, TRANSLATE_MAIN } from './mock-api'

const card = (page: Page) => page.getByLabel('Упражнение')
const field = (page: Page) => page.getByRole('textbox', { name: 'Твой перевод' })
const status = (page: Page) => page.getByRole('status')
const second = { ...TRANSLATE_EXERCISE, id: 62, prompt: 'Кот видел собаку в парке.', answer: 'The cat saw the dog in the park.', accept: ['The cat saw the dog in the park.'] }

test.describe('упражнение «перевод»', () => {
  test('напечатать перевод → Enter → «Верно» → Пробел → следующее', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE, second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Мы едим красные яблоки каждое утро.')).toBeVisible()
    await expect(card(page).getByText('Перевод', { exact: true })).toBeVisible()
    await expect(field(page)).toBeFocused()
    await page.keyboard.type('we EAT red apples, every morning')
    await page.keyboard.press('Enter')
    await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
    await expect(status(page).getByText('Так тоже можно:')).toBeVisible()
    await expect(status(page).getByText('Every morning we eat red apples.')).toBeVisible()
    expect(answers[0]).toMatchObject({ exerciseId: 61, given: 'we EAT red apples, every morning' })
    await expect(field(page)).toHaveAttribute('readonly', '')
    await page.keyboard.press('Space')
    await expect(card(page).getByText('Кот видел собаку в парке.')).toBeVisible()
    await expect(field(page)).toBeFocused()
  })

  test('опечатка в одну букву: «Почти верно», подчёркнутая буква и «Правильно»', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE] } })
    await page.goto('/exercises')
    await field(page).fill('We eat red aples every morning.')
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(status(page).getByText(/Почти верно/)).toBeVisible()
    await expect(status(page).locator('mark')).toHaveText('p')
    await expect(status(page).getByText(/Правильно:/)).toContainText(TRANSLATE_MAIN)
    await expect(status(page).getByText('Неверно')).toHaveCount(0)
    await expect(status(page).locator('[data-mistake]')).toHaveCount(0)
  })

  test('неверный ответ: «Неверно», верный ответ, ошибка вернётся завтра', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE] } })
    await page.goto('/exercises')
    await field(page).fill('They drink green tea')
    await page.keyboard.press('Enter')
    await expect(status(page).getByText('Неверно', { exact: true })).toBeVisible()
    await expect(status(page).getByText(/Верный ответ:/)).toContainText(TRANSLATE_MAIN)
    await expect(status(page).getByText('Вернётся завтра')).toBeVisible()
  })

  test('Esc очищает, пустой ответ не отправляется, Shift+Enter переносит', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE] } })
    await page.goto('/exercises')
    await page.keyboard.press('Enter')
    expect(answers).toHaveLength(0)
    await page.keyboard.type('we eat')
    await page.keyboard.press('Shift+Enter')
    await page.keyboard.type('red apples')
    expect(answers).toHaveLength(0)
    await expect(field(page)).toHaveValue('we eat\nred apples')
    await page.keyboard.press('Escape')
    await expect(field(page)).toHaveValue('')
    await expect(page.getByRole('button', { name: 'Проверить' })).toBeDisabled()
  })

  test('движение: мышью включено, с клавиатуры и при «уменьшить движение» выключено', async ({ browser }) => {
    for (const { reduced, pointer, motion } of [
      { reduced: 'no-preference' as const, pointer: true, motion: 'on' },
      { reduced: 'no-preference' as const, pointer: false, motion: 'off' },
      { reduced: 'reduce' as const, pointer: true, motion: 'off' },
    ]) {
      const context = await browser.newContext({ reducedMotion: reduced })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE] } })
      await page.goto('/exercises')
      await field(page).fill('we eat red apples every morning')
      if (pointer) await page.getByRole('button', { name: 'Проверить' }).click()
      else await page.keyboard.press('Enter')
      await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
      await expect(card(page).locator('[data-motion]')).toHaveAttribute('data-motion', motion)
      await context.close()
    }
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, exercises: { queue: [TRANSLATE_EXERCISE, { ...TRANSLATE_EXERCISE, id: 63 }, { ...TRANSLATE_EXERCISE, id: 64 }] } })
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/translate-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await page.goto('/exercises')
        await expect(card(page).getByText('Мы едим красные яблоки каждое утро.')).toBeVisible()
        await shot('empty')
        await page.keyboard.type('We eat red apples')
        await shot('typing')
        await page.keyboard.type(' every morning')
        await page.keyboard.press('Enter')
        await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
        await shot('right')
        await page.keyboard.press('Space')
        await field(page).fill('We eat red aples every morning.')
        await page.keyboard.press('Enter')
        await expect(status(page).getByText(/Почти верно/)).toBeVisible()
        await shot('typo')
        await page.keyboard.press('Space')
        await field(page).fill('They drink green tea')
        await page.keyboard.press('Enter')
        await expect(status(page).getByText('Неверно', { exact: true })).toBeVisible()
        await shot('wrong')
        await context.close()
      })
    }
  }
})
