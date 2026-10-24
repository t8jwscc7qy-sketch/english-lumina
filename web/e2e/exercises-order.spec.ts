import { expect, test } from '@playwright/test'
import { mockApi, ORDER_EXERCISE } from './mock-api'

const card = (page: import('@playwright/test').Page) => page.getByLabel('Упражнение')
const second = { ...ORDER_EXERCISE, id: 32, prompt: 'Кот видел собаку.', translation: 'Кот видел собаку.', tiles: ['the', 'saw', 'cat', 'the', 'dog'], topic: 'skill:order', answer: 'The cat saw the dog.', accept: ['The cat saw the dog.'] }

test.describe('упражнение «порядок слов»', () => {
  test('собрать предложение клавишами → «Верно» → Пробел → следующее', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [ORDER_EXERCISE, second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Вчера я пошёл домой.')).toBeVisible()
    await expect(card(page).getByText('Порядок слов')).toBeVisible()
    await page.keyboard.press('3') // yesterday
    await page.keyboard.press('4') // I
    await page.keyboard.press('2') // went
    await page.keyboard.press('Enter') // не все плитки: ничего
    expect(answers).toHaveLength(0)
    await page.keyboard.press('1') // home
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers[0]).toMatchObject({ exerciseId: 31, given: 'yesterday I went home' })
    await page.keyboard.press('Space')
    await expect(card(page).getByText('Кот видел собаку.')).toBeVisible()
  })

  test('альтернативный порядок засчитывается, неверный показывает правильный', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [ORDER_EXERCISE, { ...ORDER_EXERCISE, id: 33 }] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Вчера я пошёл домой.')).toBeVisible()
    for (const k of ['4', '2', '1', '3']) await page.keyboard.press(k) // I went home yesterday
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    await page.keyboard.press('Space')
    await expect(page.getByText('Верно', { exact: true })).toHaveCount(0)
    for (const k of ['1', '2', '3', '4']) await page.keyboard.press(k)
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status').getByText('Неверно', { exact: true })).toBeVisible()
    await expect(page.getByText('Верный ответ:')).toContainText('Yesterday I went home.')
  })

  test('повторяющиеся плитки, Backspace и Esc', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Кот видел собаку.')).toBeVisible()
    await page.keyboard.press('1')
    await page.keyboard.press('Backspace')
    await page.keyboard.press('3')
    await page.keyboard.press('Escape')
    for (const k of ['1', '3', '2', '4', '5']) await page.keyboard.press(k) // the cat saw the dog
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers[0].given).toBe('the cat saw the dog')
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, exercises: { queue: [ORDER_EXERCISE, { ...ORDER_EXERCISE, id: 34 }] } })
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/order-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await page.goto('/exercises')
        await expect(card(page).getByText('Вчера я пошёл домой.')).toBeVisible()
        await shot('empty')
        await page.keyboard.press('4')
        await page.keyboard.press('2')
        await shot('partial')
        await page.keyboard.press('1')
        await page.keyboard.press('3')
        await page.keyboard.press('Enter')
        await expect(page.getByText('Верно', { exact: true })).toBeVisible()
        await shot('right')
        await page.keyboard.press('Space')
        await expect(page.getByText('Верно', { exact: true })).toHaveCount(0) // новая карточка (текст перевода тот же)
        for (const k of ['1', '2', '3', '4']) await page.keyboard.press(k)
        await page.keyboard.press('Enter')
        await expect(page.getByText('Неверно', { exact: true })).toBeVisible()
        await shot('wrong')
        await context.close()
      })
    }
  }
})
