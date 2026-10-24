import { expect, test, type Page } from '@playwright/test'
import { EXERCISES, mockApi, type ExerciseFixture, type MistakeFixture } from './mock-api'

const SEEDED: MistakeFixture[] = [
  { id: 1, topic: 'grammar:articles', kind: 'gap', prompt: 'He is _____ honest man.', given: 'a', answer: 'an', streak: 1, dueInDays: -1 },
  { id: 2, topic: 'grammar:articles', kind: 'gap', prompt: 'She bought _____ umbrella yesterday.', given: 'the', answer: 'an', streak: 0, dueInDays: 1 },
  { id: 3, topic: 'grammar:past_simple', kind: 'gap', prompt: 'Yesterday she _____ to the cinema with her sister.', given: 'goes', answer: 'went', streak: 0, dueInDays: 3 },
]

const REPEAT_A: ExerciseFixture = { ...EXERCISES[1], id: 21, repeat: true }
const REPEAT_B: ExerciseFixture = { ...EXERCISES[0], id: 22, repeat: true }
const exercise = (page: Page) => page.getByLabel('Упражнение')
const result = (page: Page) => page.getByRole('status')

test.describe('банк ошибок', () => {
  test('пустая страница', async ({ page }) => {
    await mockApi(page, { loggedIn: true })
    await page.goto('/mistakes')
    await expect(page.getByRole('heading', { name: 'Ошибки', level: 1 })).toBeVisible()
    await expect(page.getByText('Пока ошибок нет')).toBeVisible()
    await expect(page.getByText('Открыто', { exact: true })).toBeVisible()
  })

  test('страница с данными: числа, темы, строки и срок', async ({ page }) => {
    await mockApi(page, { loggedIn: true, mistakes: SEEDED, closedThisWeek: 4 })
    await page.goto('/mistakes')
    await expect(page.getByRole('heading', { name: 'Артикли' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Прошедшее простое' })).toBeVisible()
    await expect(page.getByText('Открыто', { exact: true }).locator('xpath=..')).toContainText('3')
    await expect(page.getByText('Ждут повтора сейчас').locator('xpath=..')).toContainText('1')
    await expect(page.getByText('Закрыто за неделю').locator('xpath=..')).toContainText('4')
    const first = page.getByRole('listitem').filter({ hasText: 'He is _____ honest man.' })
    await expect(first).toContainText('Твой ответ: a')
    await expect(first).toContainText('Верно: an')
    await expect(first).toContainText('ждёт повтора')
    await expect(first).toContainText('Серия 1 из 2')
    await expect(page.getByRole('listitem').filter({ hasText: 'umbrella' })).toContainText('вернётся завтра')
    await expect(page.getByRole('listitem').filter({ hasText: 'cinema' })).toContainText('через 3 дн.')
  })

  test('неверный ответ → «Вернётся завтра» → ошибка в списке', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { queue: [EXERCISES[1]] } })
    await page.goto('/exercises')
    await expect(exercise(page).getByText('He is _____ honest man.')).toBeVisible()
    await expect(page.getByText('Повтор ошибки')).toHaveCount(0)
    await page.keyboard.press('2') // a
    await expect(result(page).getByText('Неверно', { exact: true })).toBeVisible()
    await expect(page.getByText('Вернётся завтра')).toBeVisible()
    await page.goto('/mistakes')
    const row = page.getByRole('listitem').filter({ hasText: 'honest man' })
    await expect(row).toContainText('Твой ответ: a')
    await expect(row).toContainText('Верно: an')
    await expect(row).toContainText('вернётся завтра')
    await expect(row).toContainText('Серия 0 из 2')
  })

  test('повтор: верно → «Ещё один верный ответ…» → следующий повтор', async ({ page }) => {
    await mockApi(page, {
      loggedIn: true,
      exercises: { queue: [REPEAT_A, REPEAT_B] },
      mistakes: [
        { ...SEEDED[0], exerciseId: 21, streak: 0 },
        { ...SEEDED[2], exerciseId: 22 },
      ],
    })
    await page.goto('/exercises')
    await expect(exercise(page).getByText('He is _____ honest man.')).toBeVisible()
    await expect(page.getByText('Повтор ошибки')).toBeVisible()
    await page.keyboard.press('1') // an
    await expect(result(page).getByText('Верно', { exact: true })).toBeVisible()
    await expect(page.getByText('Ещё один верный ответ, и ошибка закрыта')).toBeVisible()
    await page.keyboard.press('Space')
    await expect(page.getByText('Ещё один верный ответ, и ошибка закрыта')).toHaveCount(0)
    await expect(exercise(page).getByText('Yesterday she _____ to the cinema')).toBeVisible()
    await expect(page.getByText('Повтор ошибки')).toBeVisible()
    await page.keyboard.press('1') // went
    await expect(result(page).getByText('Верно', { exact: true })).toBeVisible()
    await expect(page.getByText('Ещё один верный ответ, и ошибка закрыта')).toBeVisible()
  })

  test('второй верный повтор закрывает ошибку и убирает её из списка', async ({ page }) => {
    await mockApi(page, {
      loggedIn: true,
      exercises: { queue: [REPEAT_A] },
      mistakes: [{ ...SEEDED[0], exerciseId: 21 }, { ...SEEDED[1], exerciseId: 99 }],
    })
    await page.goto('/exercises')
    await expect(page.getByText('Повтор ошибки')).toBeVisible()
    await page.keyboard.press('1')
    await expect(page.getByText('Ошибка закрыта', { exact: true })).toBeVisible()
    await page.goto('/mistakes')
    await expect(page.getByText('honest man')).toHaveCount(0)
    await expect(page.getByText('umbrella')).toBeVisible()
    await expect(page.getByText('Закрыто за неделю').locator('xpath=..')).toContainText('1')
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        const long: MistakeFixture = { id: 4, topic: 'skill:order', kind: 'order', prompt: 'Вчера мы очень долго ждали автобус на остановке возле старого вокзала.', given: 'yesterday we waited very long for the bus at the stop near the old station', answer: 'Yesterday we waited for the bus at the stop near the old station for a very long time.', streak: 0, dueInDays: 2 }
        await mockApi(page, {
          loggedIn: true, mistakes: [{ ...SEEDED[0], exerciseId: 21 }, ...SEEDED.slice(1), long], closedThisWeek: 4,
          exercises: { queue: [{ ...REPEAT_A }, { ...EXERCISES[0], id: 23 }] },
        })
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/mistakes-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await page.goto('/mistakes')
        await expect(page.getByRole('heading', { name: 'Артикли' })).toBeVisible()
        await shot('data')
        await page.goto('/exercises')
        await expect(page.getByText('Повтор ошибки')).toBeVisible()
        await shot('repeat')
        await page.keyboard.press('1')
        await expect(page.getByText('Ошибка закрыта', { exact: true })).toBeVisible()
        await shot('repeat-result')
        await context.close()
      })
    }
    test(`снимок пустой страницы ${theme}`, async ({ browser }) => {
      const context = await browser.newContext({ viewport: { width: 390, height: 844 }, colorScheme: theme, reducedMotion: 'reduce' })
      const page = await context.newPage()
      await mockApi(page, { loggedIn: true })
      await page.goto('/mistakes')
      await expect(page.getByText('Пока ошибок нет')).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
      await page.screenshot({ path: `test-results/screens/mistakes-empty-${theme}-390.png`, fullPage: true })
      await context.close()
    })
  }
})
