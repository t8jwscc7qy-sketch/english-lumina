import { expect, test, type Page } from '@playwright/test'
import { mockApi } from './mock-api'

const exercise = (page: Page) => page.getByLabel('Упражнение')
const status = (page: Page) => page.getByRole('status')

/** Ответить на карточку клавишами: Пробел открывает ответ, 3 — «Хорошо». */
async function rateCard(page: Page, label: string) {
  await expect(page.getByText(label)).toBeVisible()
  await page.keyboard.press('Space')
  await page.keyboard.press('3')
}

test.describe('«Сегодня» и сессия', () => {
  test('кнопка «Сегодня» называется по прогрессу и ведёт на /session', async ({ page }) => {
    for (const [minutes, name] of [[0, 'Начать сессию'], [4, 'Продолжить сессию'], [10, 'Позаниматься ещё']] as const) {
      await mockApi(page, { loggedIn: true, progress: { minutes } })
      await page.goto('/')
      const link = page.getByRole('link', { name })
      await expect(link).toBeVisible()
      await expect(link).toHaveAttribute('href', '/session')
      await page.unrouteAll({ behavior: 'ignoreErrors' })
    }
  })

  test('«Сегодня»: серия, заморозка и минуты из прогресса', async ({ page }) => {
    await mockApi(page, { loggedIn: true, progress: { minutes: 4.3, streakDays: 5, freezeUsedOn: '2026-10-24' } })
    await page.goto('/')
    await expect(page.getByText('5 дней')).toBeVisible()
    await expect(page.getByText('Заморозка спасла 24 октября')).toBeVisible()
    await expect(page.getByText(/4,3\s+из\s+10\s+мин/)).toBeVisible()
  })

  test('сессия целиком: карточки → упражнения → минимум → итог', async ({ page }) => {
    const { reviews, answers } = await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner'], progress: { step: 3 } })
    await page.goto('/')
    await page.getByRole('link', { name: 'Начать сессию' }).click()
    await expect(page).toHaveURL(/\/session$/)
    await rateCard(page, 'Карточка 1 из 2')
    await rateCard(page, 'Карточка 2 из 2')
    await expect(exercise(page)).toBeVisible()
    expect(reviews).toHaveLength(2)
    await expect(page.getByText(/6\s+из\s+10\s+мин/)).toBeVisible()

    await page.keyboard.press('1') // went
    await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
    await page.keyboard.press('Space')
    await expect(status(page).getByText('Верно', { exact: true })).toHaveCount(0)
    await page.keyboard.press('1') // an: минут станет 12
    await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
    expect(answers).toHaveLength(2)
    // результат остаётся на экране, пока не нажато «Дальше»; экран минимума после него
    await expect(page.getByRole('heading', { name: 'Минимум на сегодня выполнен' })).toHaveCount(0)
    await page.keyboard.press('Space')
    await expect(page.getByRole('heading', { name: 'Минимум на сегодня выполнен' })).toBeVisible()
    await page.getByRole('button', { name: 'Закончить' }).click()
    await expect(page.getByRole('heading', { name: 'Итог дня' })).toBeVisible()
    await expect(page.getByText('2 ответа, 2 верных')).toBeVisible()
    await expect(page.getByText('Карточек повторено: 2')).toBeVisible()
    await expect(page.getByText('Серия: 3 дня')).toBeVisible()
    await expect(page.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/')
  })

  test('«Закончить» посреди карточек ведёт к итогу и не теряет оценки', async ({ page }) => {
    const { reviews } = await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner'] })
    await page.goto('/session')
    await rateCard(page, 'Карточка 1 из 2')
    await expect(page.getByText('Карточка 2 из 2')).toBeVisible()
    await page.getByRole('button', { name: 'Закончить' }).click()
    await expect(page.getByRole('heading', { name: 'Итог дня' })).toBeVisible()
    expect(reviews).toHaveLength(1)
    await page.getByRole('button', { name: 'Ещё немного' }).click()
    await expect(page.getByText('Карточка 2 из 2')).toBeVisible()
  })

  test('нет карточек и упражнений: сообщение и «На главную»', async ({ page }) => {
    await mockApi(page, { loggedIn: true, exercises: { none: true } })
    await page.goto('/session')
    await expect(page.getByRole('heading', { name: 'Подходящих упражнений пока нет' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/')
  })

  test('ошибка прогресса: сообщение и «Повторить», упражнения всё равно идут', async ({ page }) => {
    await mockApi(page, { loggedIn: true, progress: { fail: true } })
    await page.goto('/session')
    await expect(page.getByRole('alert')).toContainText('Внутренняя ошибка')
    await expect(page.getByRole('button', { name: 'Повторить' })).toBeVisible()
    await expect(exercise(page)).toBeVisible()
  })

  for (const theme of ['light', 'dark'] as const) {
    for (const size of [{ name: '1440', width: 1440, height: 900 }, { name: '1024', width: 1024, height: 768 }, { name: '390', width: 390, height: 844 }]) {
      test(`снимки ${theme} ${size.name}`, async ({ browser }) => {
        const context = await browser.newContext({ viewport: { width: size.width, height: size.height }, colorScheme: theme, reducedMotion: 'reduce' })
        const page = await context.newPage()
        await mockApi(page, { loggedIn: true, dueCards: ['run', 'runner'], progress: { step: 3, freezeUsedOn: '2026-10-24' } })
        const shot = async (name: string) => {
          const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
          expect(overflow, name).toBeLessThanOrEqual(0)
          await page.screenshot({ path: `test-results/screens/session-${name}-${theme}-${size.name}.png`, fullPage: true })
        }
        await page.goto('/')
        await expect(page.getByText('3 дня')).toBeVisible()
        await shot('today')
        await page.getByRole('link', { name: 'Начать сессию' }).click()
        await expect(page.getByText('Карточка 1 из 2')).toBeVisible()
        await shot('cards')
        await page.keyboard.press('Space')
        await page.keyboard.press('3')
        await rateCard(page, 'Карточка 2 из 2')
        await expect(exercise(page)).toBeVisible()
        await shot('exercise')
        await page.keyboard.press('1')
        await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
        await page.keyboard.press('Space')
        await expect(status(page).getByText('Верно', { exact: true })).toHaveCount(0)
        await page.keyboard.press('1')
        await expect(status(page).getByText('Верно', { exact: true })).toBeVisible()
        await page.keyboard.press('Space')
        await expect(page.getByRole('heading', { name: 'Минимум на сегодня выполнен' })).toBeVisible()
        await shot('minimum')
        await page.getByRole('button', { name: 'Закончить' }).click()
        await expect(page.getByRole('heading', { name: 'Итог дня' })).toBeVisible()
        await shot('summary')
        await context.close()
      })
    }
  }
})
