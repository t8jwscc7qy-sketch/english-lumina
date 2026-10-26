import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

test.describe('курс «Путь»', () => {
  test('после входа открывается «Путь», урок проходится от «Начать» до итога', async ({ page }) => {
    const { lessonCompletes } = await mockApi(page, { loggedIn: true })
    await page.goto('/')
    await expect(page).toHaveURL(/\/path$/)
    await expect(page.getByLabel('Урок 2: Как дела?, закрыт')).toBeVisible()
    await page.getByRole('link', { name: 'Начать', exact: true }).click()
    await expect(page).toHaveURL(/\/lesson$/)

    // Слова: Enter — «Дальше»
    await expect(page.getByRole('heading', { name: 'hello' })).toBeVisible()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('heading', { name: 'I', exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Дальше' }).click()

    // Текст: слово по нажатию
    await page.getByRole('button', { name: 'Tom' }).first().click()
    await expect(page.getByText('— имя')).toBeVisible()
    await page.getByRole('button', { name: 'Дальше' }).click()

    // Вопрос с клавиатуры
    await page.keyboard.press('1')
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status', { name: 'Результат' })).toContainText('Верно!')
    await page.keyboard.press('Enter')

    // Пропуск: ошибка, задание вернётся в конце
    await page.getByRole('button', { name: 'I', exact: true }).click()
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(page.getByRole('status', { name: 'Результат' })).toContainText('Правильно: Hello')
    await page.getByRole('button', { name: 'Дальше' }).click()

    // Порядок слов
    for (const w of ['I', 'am', 'Tom']) await page.getByRole('button', { name: w, exact: true }).click()
    await page.getByRole('button', { name: 'Проверить' }).click()
    await page.getByRole('button', { name: 'Дальше' }).click()

    // Пары
    for (const [en, ru] of [['hello', 'привет'], ['I', 'я'], ['am', 'есть']]) {
      await page.getByRole('button', { name: en, exact: true }).click()
      await page.getByRole('button', { name: ru, exact: true }).click()
    }
    await page.getByRole('button', { name: 'Дальше' }).click()

    // Повтор пропуска
    await expect(page.getByText('Ещё раз')).toBeVisible()
    await page.getByRole('button', { name: /Hello/ }).click()
    await page.getByRole('button', { name: 'Проверить' }).click()
    await page.getByRole('button', { name: 'Завершить' }).click()

    await expect(page.getByRole('heading', { name: 'Урок пройден' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Следующий урок: Как дела?' })).toBeEnabled()
    expect(lessonCompletes).toHaveLength(1)
    expect(lessonCompletes[0]).toMatchObject({ lessonId: 'u1-l1', mistakes: 1 })

    await page.getByRole('link', { name: 'К пути' }).click()
    await expect(page.getByText('Урок 2: Как дела?')).toBeVisible()
  })

  const SIZES = [
    { name: '1440', width: 1440, height: 900 },
    { name: '390', width: 390, height: 844 },
  ]
  for (const theme of ['light', 'dark'] as const) {
    for (const size of SIZES) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true })
        await page.goto('/path')
        await expect(page.getByText('Урок 1: Привет')).toBeVisible()
        await page.screenshot({ path: `test-results/screens/path-${theme}-${size.name}.png`, fullPage: true })
        await page.goto('/lesson')
        await expect(page.getByRole('heading', { name: 'hello' })).toBeVisible()
        await page.screenshot({ path: `test-results/screens/lesson-word-${theme}-${size.name}.png` })
        await page.keyboard.press('Enter')
        await page.keyboard.press('Enter')
        await page.getByRole('button', { name: 'Hello' }).first().click()
        await page.screenshot({ path: `test-results/screens/lesson-text-${theme}-${size.name}.png`, fullPage: true })
        await page.getByRole('button', { name: 'Дальше' }).click()
        await page.keyboard.press('2')
        await page.keyboard.press('Enter')
        await page.screenshot({ path: `test-results/screens/lesson-wrong-${theme}-${size.name}.png` })
        await context.close()
      })
    }
  }
})
