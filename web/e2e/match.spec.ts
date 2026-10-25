import { expect, test, type Page } from '@playwright/test'
import { MATCH_EXERCISE, mockApi } from './mock-api'

const card = (page: Page) => page.getByLabel('Упражнение')
const word = (page: Page, i: number) => page.getByRole('button', { name: new RegExp(`^${i + 1}\\. `) })
const option = (page: Page, j: number) => page.getByRole('button', { name: new RegExp(`^${'abcde'[j]}\\. `) })
const second = { ...MATCH_EXERCISE, id: 52 }
// верные пары упражнения: apple→b, house→e, water→d, book→a, dog→c
const RIGHT: [number, number][] = [[0, 1], [1, 4], [2, 3], [3, 0], [4, 2]]

async function pairByMouse(page: Page, pairs: [number, number][]) {
  for (const [w, o] of pairs) {
    await word(page, w).click()
    await option(page, o).click()
  }
}

test.describe('упражнение «сопоставление»', () => {
  test('мышью: пять пар, нити, «Проверить», пять из пяти', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [MATCH_EXERCISE, second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Соедини слова с переводами')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Проверить' })).toBeDisabled()
    await pairByMouse(page, RIGHT)
    await expect(page.locator('path.m-thread')).toHaveCount(5)
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(page.getByRole('status').getByText('Все пары верны')).toBeVisible()
    expect(answers[0]).toMatchObject({ exerciseId: 51, given: 'apple=яблоко;house=дом;water=вода;book=книга;dog=собака' })
    await expect(page.locator('.m-confetti')).toHaveCount(12)
    await expect(page.locator('.fox-jump')).toHaveCount(1)
    await expect(page.getByText('Счёт: 5 / 5')).toBeVisible()
  })

  test('мышью: неверные пары трясутся, разбор называет верный перевод', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [MATCH_EXERCISE] } })
    await page.goto('/exercises')
    // dog и book перепутаны
    await pairByMouse(page, [[0, 1], [1, 4], [2, 3], [3, 2], [4, 0]])
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(page.getByRole('status').getByText('Есть ошибки в парах')).toBeVisible()
    await expect(page.locator('.m-wrong')).toHaveCount(2)
    await expect(page.locator('.m-spark')).toHaveCount(12)
    await expect(page.locator('.m-confetti')).toHaveCount(0)
    const report = page.getByRole('list', { name: 'Разбор пар' })
    await expect(report).toContainText('book')
    await expect(report).toContainText('верный перевод: книга')
  })

  test('клавиатурой всё без анимаций: цифры, буквы, Backspace, Enter, Пробел', async ({ page }) => {
    const { answers } = await mockApi(page, { loggedIn: true, exercises: { queue: [MATCH_EXERCISE, second] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Соедини слова с переводами')).toBeVisible()
    await page.keyboard.press('1')
    await page.keyboard.press('a') // ошибочно: книга
    await page.keyboard.press('Backspace')
    for (const [w, o] of RIGHT) {
      await page.keyboard.press(String(w + 1))
      await page.keyboard.press('abcde'[o])
    }
    await expect(page.locator('[data-motion="off"]')).toHaveCount(1)
    await page.keyboard.press('Enter')
    await expect(page.getByRole('status').getByText('Все пары верны')).toBeVisible()
    expect(answers).toHaveLength(1)
    for (const cls of ['.m-flash', '.m-spark', '.m-wrong', '.m-confetti', '.m-ring', '.fox-jump']) await expect(page.locator(cls)).toHaveCount(0)
    expect(await page.evaluate(() => document.getAnimations().filter((a) => a.playState === 'running').length)).toBe(0)
    await page.keyboard.press('Space')
    await expect(page.getByRole('button', { name: 'Проверить' })).toBeDisabled() // следующее упражнение
    await expect(page.locator('.m-tile-in')).toHaveCount(0) // после клавиатуры входа нет
  })

  test('«уменьшить движение»: ни одной анимации ни до, ни после проверки', async ({ browser }) => {
    const context = await browser.newContext({ reducedMotion: 'reduce' })
    const page = await context.newPage()
    await mockApi(page, { loggedIn: true, exercises: { queue: [MATCH_EXERCISE] } })
    await page.goto('/exercises')
    await expect(card(page).getByText('Соедини слова с переводами')).toBeVisible()
    expect(await page.evaluate(() => document.getAnimations().filter((a) => a.playState === 'running').length)).toBe(0)
    await pairByMouse(page, RIGHT)
    await page.getByRole('button', { name: 'Проверить' }).click()
    await expect(page.getByRole('status').getByText('Все пары верны')).toBeVisible()
    for (const cls of ['.m-flash', '.m-spark', '.m-confetti', '.m-ring', '.fox-jump']) await expect(page.locator(cls)).toHaveCount(0)
    expect(await page.evaluate(() => document.getAnimations().filter((a) => a.playState === 'running').length)).toBe(0)
    await context.close()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, exercises: { queue: [MATCH_EXERCISE, { ...MATCH_EXERCISE, id: 53 }] } })
        await page.goto('/exercises')
        await expect(card(page).getByText('Соедини слова с переводами')).toBeVisible()
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/match-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await shot('empty')
        await pairByMouse(page, RIGHT.slice(0, 3))
        await shot('partial')
        await pairByMouse(page, RIGHT.slice(3))
        await shot('full')
        await page.getByRole('button', { name: 'Проверить' }).click()
        await expect(page.getByRole('status').getByText('Все пары верны')).toBeVisible()
        await shot('right')
        await page.keyboard.press('Space')
        await pairByMouse(page, [[0, 1], [1, 4], [2, 3], [3, 2], [4, 0]])
        await page.getByRole('button', { name: 'Проверить' }).click()
        await expect(page.getByRole('status').getByText('Есть ошибки в парах')).toBeVisible()
        await shot('wrong')
        await context.close()
      })
    }
  }
})
