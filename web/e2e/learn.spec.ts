import { expect, test } from '@playwright/test'
import { mockApi } from './mock-api'

const WORDS = [
  { lexemeId: 1, lemma: 'apple', pos: 'noun', level: 'A1', translations: ['яблоко'], ipa: 'ˈæpəl', rank: 1200 },
  { lexemeId: 2, lemma: 'run', pos: 'verb', level: 'A1', translations: ['бежать'] },
]

test.describe('знакомство', () => {
  test('«Сегодня»: карточка знакомства ведёт на слова', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: { words: WORDS } })
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Знакомство' })).toBeVisible()
    await expect(page.getByText('Новых слов сегодня: 1 из 5')).toBeVisible()
    await page.getByRole('link', { name: 'Начать знакомство' }).click()
    await expect(page).toHaveURL(/\/learn$/)
    await expect(page.getByRole('heading', { name: 'apple' })).toBeVisible()
  })

  test('проход блока мышью и ввод слов', async ({ page }) => {
    const introduced: unknown[] = []
    await mockApi(page, { loggedIn: true, learn: { words: WORDS, introduced } })
    await page.goto('/learn')
    await expect(page.getByText('Слово 1 из 2')).toBeVisible()
    await expect(page.getByText('яблоко')).toBeVisible()
    await expect(page.locator('mark')).toHaveText('apple')
    await page.screenshot({ path: 'test-results/screens/learn-word.png' })
    await page.getByRole('button', { name: 'Дальше' }).click()
    await expect(page.getByRole('heading', { name: 'run' })).toBeVisible()
    await page.getByRole('button', { name: 'Начать повторять' }).click()
    await expect(page).toHaveURL(/\/cards$/)
    expect(introduced).toHaveLength(1)
    expect(introduced[0]).toMatchObject({ lexemeIds: [1, 2] })
  })

  test('клавиатура: стрелки листают слова', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: { words: WORDS } })
    await page.goto('/learn')
    await expect(page.getByRole('heading', { name: 'apple' })).toBeVisible()
    await page.keyboard.press('ArrowRight')
    await expect(page.getByRole('heading', { name: 'run' })).toBeVisible()
    await page.keyboard.press('ArrowLeft')
    await expect(page.getByRole('heading', { name: 'apple' })).toBeVisible()
  })

  test('нет слов и исчерпанный лимит — спокойные экраны', async ({ page }) => {
    await mockApi(page, { loggedIn: true, learn: {} })
    await page.goto('/learn')
    await expect(page.getByRole('heading', { name: 'Слов для знакомства пока нет' })).toBeVisible()
    await mockApi(page, { loggedIn: true, learn: { words: WORDS, exhausted: true } })
    await page.goto('/learn')
    await expect(page.getByRole('heading', { name: 'На сегодня всё' })).toBeVisible()
  })

  test('быстрый тест: отметка «знаю» и итог', async ({ page }) => {
    const placed: unknown[] = []
    await mockApi(page, {
      loggedIn: true,
      learn: { placement: [{ lexemeId: 1, lemma: 'apple', pos: 'noun' }, { lexemeId: 2, lemma: 'run', pos: 'verb' }], placementNext: 'learn', placed },
    })
    await page.goto('/learn/placement')
    await page.getByRole('checkbox', { name: /apple/ }).check()
    await page.getByRole('button', { name: /Готово/ }).click()
    await expect(page.getByRole('heading', { name: 'Отмечено слов: 1' })).toBeVisible()
    expect(placed[0]).toMatchObject({ known: [1] })
    await page.getByRole('link', { name: 'К знакомству' }).click()
    await expect(page).toHaveURL(/\/learn$/)
  })

  test('настройки: блок «Обучение» сохраняет значения', async ({ page }) => {
    const prefsPuts: unknown[] = []
    await mockApi(page, { loggedIn: true, learn: { prefsPuts } })
    await page.goto('/settings')
    await page.getByRole('radio', { name: 'Столько, сколько я укажу' }).check()
    await page.getByLabel('Слов в день').fill('9')
    await page.getByRole('button', { name: 'Применить' }).click()
    await expect(page.getByText('Сохранено')).toBeVisible()
    expect(prefsPuts[0]).toEqual({ newWordsMode: 'manual', newWordsPerDay: 9, campCoverage: 0.95 })
  })

  for (const size of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    test(`вид слова ${size.width}px без горизонтальной прокрутки`, async ({ page }) => {
      await page.setViewportSize(size)
      await mockApi(page, { loggedIn: true, learn: { words: WORDS } })
      await page.goto('/learn')
      await expect(page.getByRole('heading', { name: 'apple' })).toBeVisible()
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
      expect(overflow).toBe(false)
      await page.screenshot({ path: `test-results/screens/learn-${size.width}.png` })
    })
  }
})
