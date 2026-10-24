import { expect, test } from '@playwright/test'

// Против настоящего Go-сервера (и dev-сервера Vite с прокси). Запуск: REAL_API=http://127.0.0.1:5199 npm run e2e -- real-api
// Пароль настоящего пользователя тесту не нужен: проверяется путь ошибки.
test.skip(
  !process.env.REAL_API,
  'нужен запущенный Go-сервер: задай REAL_API=адрес dev-сервера Vite',
)
test.use({ baseURL: process.env.REAL_API })

test('настоящий сервер: гость попадает на вход, неверный пароль даёт сообщение сервера', async ({
  page,
}) => {
  await page.goto('/')
  await expect(page).toHaveURL(/\/login$/)

  await page.getByLabel('Логин').fill('firimon')
  await page.getByLabel('Пароль').fill('definitely-wrong-password')
  await page.getByRole('button', { name: 'Войти' }).click()

  await expect(page.getByRole('alert')).toContainText('Неверный логин или пароль')
  await expect(page.getByLabel('Пароль')).toBeFocused()
  await expect(page).toHaveURL(/\/login$/)
})

test('настоящий сервер: настройки без входа закрыты (401), а не 404 или 500', async ({ request }) => {
  const res = await request.get('/api/v1/settings')
  expect(res.status()).toBe(401)
  expect((await res.json()).error.code).toBe('unauthorized')
})
