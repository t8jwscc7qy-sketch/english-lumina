import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

const FIRST = 'Yesterday she _____ to the cinema with her sister.'

test.describe('упражнения', () => {
  test('меню → ответ клавишей → «Верно», слабые темы, Пробел → следующее', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true })
    await page.goto('/')
    await page.getByRole('link', { name: 'Упражнения', exact: true }).click()
    await expect(page).toHaveURL(/\/exercises$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Упражнения' })).toBeVisible()
    await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
    await expect(page.getByText('Вчера она ходила в кино со своей сестрой.')).toBeVisible()
    await expect(page.getByText('Пока нет данных', { exact: false })).toHaveCount(0)

    await page.keyboard.press('1') // went
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers).toHaveLength(1)
    expect(answers[0]).toMatchObject({ exerciseId: 11, given: 'went' })
    await expect(page.locator('mark')).toHaveText('went')
    // Рейтинг темы вырос (1500 → 1540) и показан в «Затронуто» и в «Слабых темах».
    await expect(page.locator('[data-affected]')).toContainText('1540')
    const weak = page.getByRole('region', { name: 'Слабые темы' })
    await expect(weak.getByText('Прошедшее простое')).toBeVisible()
    await expect(weak).toContainText('1540')

    await page.keyboard.press('Space')
    await expect(page.getByLabel('Упражнение').getByText('He is _____ honest man.')).toBeVisible()
    await expect(page.locator('mark')).toHaveCount(0)
  })

  test('неверный ответ клавишей показывает верный', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true })
    await page.goto('/exercises')
    await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
    await page.keyboard.press('2') // goes
    await expect(page.getByRole('status').getByText('Неверно', { exact: true })).toBeVisible()
    await expect(page.getByText('Верный ответ:')).toContainText('went')
    await expect(page.locator('mark')).toHaveText('went')
    expect(answers[0]).toMatchObject({ given: 'goes' })
    await expect(page.locator('[data-affected]')).toContainText('1460')
  })

  test('ручной ввод: ответ вводится и отправляется Enter', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true })
    await page.goto('/exercises')
    await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
    await page.keyboard.press('1')
    await expect(page.getByText('Верно', { exact: true })).toBeVisible()
    await page.keyboard.press('Space')
    await expect(page.getByText('He is _____ honest man.')).toBeVisible()
    await page.keyboard.press('1')
    await expect(page.getByText('Верно', { exact: true })).toBeVisible()
    await page.keyboard.press('Space')
    const input = page.getByLabel('Твой ответ')
    await expect(input).toBeFocused()
    await input.fill('at')
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers.at(-1)).toMatchObject({ exerciseId: 13, given: 'at' })
  })

  test('нет упражнений: понятное пустое состояние', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { none: true } })
    await page.goto('/exercises')
    await expect(page.getByRole('heading', { name: 'Подходящих упражнений пока нет' })).toBeVisible()
    await expect(page.getByText('Добавь слова в карточки')).toBeVisible()
    await expect(page.getByRole('link', { name: 'Добавить слова' })).toHaveAttribute('href', '/words')
    await expect(page.getByLabel('Упражнение')).toHaveCount(0)
  })

  test('ошибка 500 при ответе: «Повторить» и «Пропустить»', async ({ page }) => {
    const { answers, exState } = await mockApi(page, { loggedIn: true, exercises: { failAnswers: true } })
    await page.goto('/exercises')
    await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
    await page.keyboard.press('1')
    await expect(page.getByRole('alert')).toContainText('Внутренняя ошибка')
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Пропустить' })).toBeVisible()
    await expect(page.getByText('Верно', { exact: true })).toHaveCount(0)

    exState.failAnswers = false
    await page.getByRole('button', { name: 'Повторить' }).click()
    await expect(page.getByRole('status').getByText('Верно', { exact: true })).toBeVisible()
    expect(answers).toHaveLength(2)
    expect(answers[1].key).toBe(answers[0].key) // повтор идёт с тем же ключом
  })

  test('до ответа верного ответа нет в DOM вне списка вариантов', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    const responses: string[] = []
    page.on('response', async (r) => {
      if (r.url().includes('/exercises/next')) responses.push(await r.text())
    })
    await page.goto('/exercises')
    await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
    const leaks = await page.evaluate(() => {
      const right = 'went'
      const found: string[] = []
      for (const el of document.querySelectorAll('body *')) {
        if (el.closest('button')) continue // варианты ответа сами содержат верный
        for (const a of Array.from(el.attributes)) {
          if (a.value.toLowerCase().split(/\W+/).includes(right)) found.push(`${el.tagName}[${a.name}]`)
        }
        for (const n of Array.from(el.childNodes)) {
          if (n.nodeType === Node.TEXT_NODE && (n.textContent ?? '').toLowerCase().split(/\W+/).includes(right)) found.push(`text in ${el.tagName}`)
        }
      }
      return found
    })
    expect(leaks).toEqual([])
    await expect(page.locator('mark')).toHaveCount(0)
    expect(responses.join('')).not.toContain('"answer"')
  })
})

const SIZES = [
  { name: '1440', width: 1440, height: 900 },
  { name: '1024', width: 1024, height: 768 },
  { name: '390', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const size of SIZES) {
    test(`снимки упражнений: ${theme} ${size.name}`, async ({ browser }) => {
      const context = await browser.newContext({
        viewport: { width: size.width, height: size.height },
        colorScheme: theme,
        reducedMotion: 'reduce',
      })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true })
      const shot = async (name: string) => {
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
        expect(overflow, name).toBeLessThanOrEqual(0)
        await page.screenshot({ path: `test-results/screens/exercises-${name}-${theme}-${size.name}.png`, fullPage: true })
      }
      await page.goto('/exercises')
      await expect(page.getByLabel('Упражнение').getByText(FIRST)).toBeVisible()
      await shot('question')
      await page.keyboard.press('2')
      await expect(page.getByText('Неверно', { exact: true })).toBeVisible()
      await shot('wrong')
      await page.keyboard.press('Space')
      await expect(page.getByText('He is _____ honest man.')).toBeVisible()
      await page.keyboard.press('1')
      await expect(page.getByText('Верно', { exact: true })).toBeVisible()
      await shot('right')
      await context.close()

      const ctx2 = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
      const p2 = await ctx2.newPage()
      await mockApi(p2, { loggedIn: true, exercises: { none: true } })
      await p2.goto('/exercises')
      await expect(p2.getByRole('heading', { name: 'Подходящих упражнений пока нет' })).toBeVisible()
      await p2.screenshot({ path: `test-results/screens/exercises-empty-${theme}-${size.name}.png`, fullPage: true })
      await ctx2.close()
    })
  }
}
