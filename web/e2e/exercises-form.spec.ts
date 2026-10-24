import { expect, test } from '@playwright/test'
import { FORM_EXERCISE, mockApi } from './mock-api'

const card = (page: import('@playwright/test').Page) => page.getByLabel('Упражнение')
const second = { ...FORM_EXERCISE, id: 42, prompt: 'They _____ home late.', hint: 'come', translation: 'Они пришли домой поздно.', choices: ['came', 'comes', 'coming', 'come'], answer: 'came', word: { lemma: 'come', pos: 'verb', inCards: false } }

test.describe('упражнение «выбор формы»', () => {
  test('ответ клавишей → «Верно» → «Добавить в карточки» → Пробел → следующее', async ({ page }) => {
    const { answers, cardAdds } = await mockApi(page, { loggedIn: true, exercises: { queue: [FORM_EXERCISE, second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Yesterday she')).toBeVisible()
    await expect(card(page).getByText('начальная форма: go')).toBeAttached()
    await page.keyboard.press('1') // went
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers[0]).toMatchObject({ exerciseId: 41, given: 'went' })
    await page.getByRole('button', { name: 'Добавить в карточки' }).click()
    await expect(page.getByRole('button', { name: 'В карточках' })).toBeDisabled()
    expect(cardAdds).toEqual([{ lemma: 'go', pos: 'verb' }])
    await page.keyboard.press('Space') // фокус остался на карточке
    await expect(card(page).getByText('They')).toBeVisible()
    await expect(page.getByText('Верно', { exact: true })).toHaveCount(0)
    expect(cardAdds).toHaveLength(1)
  })

  test('неверный ответ показывает верную форму', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [FORM_EXERCISE] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Yesterday she')).toBeVisible()
    await page.keyboard.press('2') // goes
    await expect(page.getByRole('status').getByText('Неверно', { exact: true })).toBeVisible()
    await expect(page.getByText('Верный ответ:')).toContainText('went')
  })

  test('слово уже в карточках: кнопки нет', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [{ ...FORM_EXERCISE, word: { lemma: 'go', pos: 'verb', inCards: true } }] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Yesterday she')).toBeVisible()
    await page.keyboard.press('1')
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Добавить в карточки' })).toHaveCount(0)
  })

  test('409 no_examples показывает ошибку под кнопкой', async ({ page }) => {
    const { cardAdds } = await mockApi(page, { loggedIn: true, exercises: { queue: [{ ...FORM_EXERCISE, noExamples: true }] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Yesterday she')).toBeVisible()
    await page.keyboard.press('1')
    await page.getByRole('button', { name: 'Добавить в карточки' }).click()
    await expect(page.getByRole('alert')).toContainText('нет подходящих примеров')
    expect(cardAdds).toHaveLength(1)
    await expect(page.getByRole('button', { name: 'Добавить в карточки' })).toBeVisible()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, exercises: { queue: [FORM_EXERCISE, second] } })
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/form-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await page.goto('/exercises')
        await expect(card(page).getByText('Yesterday she')).toBeVisible()
        await shot('empty')
        await page.keyboard.press('1')
        await expect(page.getByText('Верно', { exact: true })).toBeVisible()
        await shot('right')
        await page.keyboard.press('Space')
        await expect(page.getByText('Верно', { exact: true })).toHaveCount(0)
        await expect(card(page).getByText('They')).toBeVisible()
        await page.keyboard.press('2') // comes
        await expect(page.getByText('Неверно', { exact: true })).toBeVisible()
        await shot('wrong')
        await context.close()
      })
    }
  }
})
