# Онбординг, настройки и счётчик срока — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Пользователь после входа проходит онбординг, настройки сохраняются в БД и редактируются, а счётчик показывает реальный срок до цели в онбординге, настройках и на «Сегодня».

**Architecture:** Контракт `GET/PUT /api/v1/settings` описывается в `openapi.yaml`, из него генерируются Go и TS. Сервер хранит и проверяет настройки; прогноз считается только на фронтенде чистой функцией `forecast`. Одна форма настроек используется и в онбординге, и в разделе «Настройки».

**Tech Stack:** Go (net/http, pgx, sqlc, oapi-codegen), React 19 + TypeScript, TanStack Query, react-router, Vitest, Playwright, Sonner. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-23-onboarding-settings-design.md`

## Global Constraints

- Контракт меняется сначала в `api/openapi.yaml`, код генерируется (`go generate ./internal/api`, `npm run gen:api` в `web/`); генерированное руками не правим.
- SQL только в `queries/`; запросы `GetSettings` и `UpsertSettings` уже есть, миграции не нужны.
- Не добавлять зависимости (нет react-hook-form и zod: форма на состоянии React).
- Интерфейс на русском, на «ты». `dangerouslySetInnerHTML` запрещён.
- Ошибки API: `{"error":{"code","message"}}`; валидация → 400 `invalid_request`; нет настроек → 404 `onboarding_required`.
- Валидация: уровни A1–C1; цель не ниже текущего; дата цели не в прошлом (по часовому поясу пользователя) и не дальше 5 лет; минуты 5–240; часовой пояс принимает `time.LoadLocation` и не пустой.
- Часы перехода (середины накопленных часов Cambridge 95/190/375/550/750): A1→A2 95, A2→B1 185, B1→B2 175, B2→C1 200. Коэффициент самостоятельных занятий 1,5 (нижняя граница) и 2,0 (верхняя). Больше 240 минут в день — «нереалистично».
- Логику прогноза держать чистой функцией с юнит-тестами. Дизайн по `DESIGN.md`, после изменений интерфейса обновить его.
- Коммитить только по просьбе пользователя; перед коммитом тесты и линтеры. Секреты не печатать.

## Review Focus

- Дата цели «сегодня» по часовому поясу пользователя допустима, «вчера» нет; граница считается в часовом поясе из настроек, а не сервера (Task 2).
- Минуты 4 и 241, уровни `a1`/`D1`/пустая строка, цель ниже уровня, неизвестный часовой пояс — все дают 400, а не 500 (Task 2, 3).
- `PUT` с лишними полями, битым JSON или телом больше лимита отвечает 400 и ничего не пишет (Task 3).
- Склонение «1 месяц / 2 месяца / 5 месяцев / 11 месяцев / 21 месяц» и «1 неделя / 2 недели / 5 недель» (Task 1).
- Цель равна уровню: прогноз «цель достигнута», без деления на ноль; дата цели сегодня или в прошлом не ломает режим «назад» (Task 1).
- Сбой `GET /settings` (500, сеть) не должен отправлять в онбординг: редирект только на 404 `onboarding_required` (Task 4).
- Двойной клик «Сохранить» не создаёт ошибку и не отправляет два запроса одновременно (Task 5).

---

### Task 1: Функция прогноза `forecast`

**Files:**
- Create: `web/src/lib/forecast.ts`
- Test: `web/src/lib/forecast.test.ts`

**Interfaces:**
- Consumes: тип `Level` и `LEVELS` из `web/src/features/today/levels.ts`.
- Produces (экспорты `forecast.ts`):
  - `type Range = { min: number; max: number }`
  - `type ForwardForecast = { kind: 'reached' } | { kind: 'estimate'; hours: Range; days: Range; finishBy: { earliest: Date; latest: Date } }`
  - `type BackwardForecast = { kind: 'reached' } | { kind: 'past' } | { kind: 'needed'; minutesPerDay: Range; unrealistic: boolean }`
  - `forecastForward(current: Level, target: Level, minutesPerDay: number, today: Date): ForwardForecast`
  - `forecastBackward(current: Level, target: Level, targetDate: Date, today: Date): BackwardForecast`
  - `describeDuration(days: Range): string` — например «6–8 месяцев», «3–5 недель», «около 2 месяцев» (если границы округляются одинаково).
  - Параметр `observedPace` (фактический темп) в v1 не добавляем (YAGNI): сигнатуру расширим, когда появятся упражнения и рейтинг.

- [ ] **Step 1: Write the failing tests** в `forecast.test.ts`:
  - `forecastForward('B1','B2',90,today)`: `hours` = `{min: 262.5, max: 350}`, `days` = `{min: 175, max: 233.33…}` (`toBeCloseTo`), `finishBy.earliest` = today + 175 дней (округление вверх до целого дня).
  - `forecastForward('B2','B2',30,today)` и текущий выше цели → `{kind:'reached'}`.
  - Многоступенчатый: `forecastForward('A1','C1',60,today).hours.min` = (95+185+175+200)×1,5.
  - `forecastBackward('B1','B2', today+200 дней, today)`: `minutesPerDay` ≈ `{min: 262.5*60/200, max: 350*60/200}`, `unrealistic: false`; для `today+30 дней` → `unrealistic: true` (больше 240).
  - `forecastBackward` с датой = сегодня и в прошлом → `{kind:'past'}`; цель = уровню → `{kind:'reached'}`.
  - `describeDuration`: `{min: 175, max: 233}` → «6–8 месяцев»; склонение месяцев по числу: 1 → «1 месяц», 2–4 → «месяца», 5, 11, 12, 14 → «месяцев», 21 → «21 месяц», 22 → «22 месяца»; меньше ~30 дней — в неделях с правильным склонением (1 неделя, 2 недели, 5 недель); одинаковые границы после округления → «около N месяцев».
  - Все тесты передают `today` явно, `new Date()` внутри функций не вызывается.

- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/forecast.test.ts` — ожидается FAIL (модуля нет).

- [ ] **Step 3: Implement** модуль по сигнатурам выше. Константы `TRANSITION_HOURS` (по уровню-источнику: A1 95, A2 185, B1 175, B2 200), `SELF_STUDY_FACTOR = { min: 1.5, max: 2.0 }`, `MAX_MINUTES_PER_DAY = 240`, `DAYS_PER_MONTH = 30.4` — с комментарием об источнике (Cambridge English, guided learning hours) и о том, что коэффициент — допущение. Месяцы: округление `Math.round`, неделя = 7 дней.

- [ ] **Step 4: Run** тот же тест — PASS; затем `npx tsc -b && npm run lint`.

- [ ] **Step 5: Commit** (по просьбе пользователя): `git add web/src/lib/forecast.ts web/src/lib/forecast.test.ts`, сообщение «Счётчик срока: чистая функция forecast».

### Task 2: Проверка настроек на сервере (чистая функция)

**Files:**
- Create: `internal/settings/validate.go`
- Test: `internal/settings/validate_test.go`

**Interfaces:**
- Produces:
  - `type Input struct { CurrentLevel, TargetLevel string; TargetDate *time.Time; DailyMinutes int; Timezone string }` (`TargetDate` — календарный день, время игнорируется).
  - `type Valid struct { CurrentLevel, TargetLevel string; TargetDate *time.Time; DailyMinutes int32; Timezone string }`
  - `func Validate(in Input, now time.Time) (Valid, error)` — ошибка типа `*ValidationError` с полем `Message string` на русском (его покажем пользователю).

- [ ] **Step 1: Write the failing tests** (таблица): корректный ввод проходит; уровень `a1`, `D1`, `""` → ошибка; цель ниже уровня → ошибка; минуты 4 и 241 → ошибка, 5 и 240 проходят; неизвестный пояс `Mars/Base` и `""` → ошибка; дата цели «вчера» в поясе `Pacific/Kiritimati` и «сегодня» там же при `now` в UTC (граница считается в поясе пользователя); дата цели через 5 лет + 1 день → ошибка, ровно 5 лет проходит; `TargetDate == nil` проходит.
- [ ] **Step 2: Run** `go test ./internal/settings/...` — FAIL.
- [ ] **Step 3: Implement** `Validate`. «Сегодня» = `now.In(loc)` обрезанное до дня; сравнение по календарным дням (год-месяц-день), не по моментам.
- [ ] **Step 4: Run** `go test -race ./internal/settings/...` — PASS; `go vet ./...`.
- [ ] **Step 5: Commit**: `internal/settings/`, «Проверка настроек обучения».

### Task 3: Контракт и обработчики `GET/PUT /api/v1/settings`

**Files:**
- Modify: `api/openapi.yaml`; `internal/httpapi/auth.go` (расширить `AuthStore` методами `GetSettings(ctx, userID int64) (db.Setting, error)` и `UpsertSettings(ctx, db.UpsertSettingsParams) (db.Setting, error)`); `internal/httpapi/health.go` при необходимости
- Create: `internal/httpapi/settings.go`, `internal/httpapi/settings_test.go`
- Regenerate: `internal/api/api.gen.go`
- Modify: тестовые фейки хранилища в `internal/httpapi/*_test.go`, если они реализуют `AuthStore`
- Modify: `internal/httpapi/integration_test.go` (добавить сценарий)

**Interfaces:**
- Consumes: `settings.Validate`, `settings.Input`, `UserFromContext`/`authenticate`.
- Produces в OpenAPI: схема `Settings` (`currentLevel`, `targetLevel` — enum A1…C1; `targetDate` — `string`, `format: date`, nullable, необязательное; `dailyMinutes` integer 5–240; `timezone` string) для запроса и ответа; операции `getSettings` и `putSettings` (оба требуют сессию, 401 как у `/auth/me`); 404 `onboarding_required` у `getSettings`; 400 `BadRequest` и 403 у `putSettings`; в enum `Error.error.code` добавить `onboarding_required`.

- [ ] **Step 1: Contract**: правка `openapi.yaml` по Interfaces; `go generate ./internal/api` (`oapi-codegen` в `~/go/bin`); `go build ./...` покажет, что `authServer` не реализует новый интерфейс.
- [ ] **Step 2: Write the failing tests** (`settings_test.go`, фейк хранилища в памяти, шаблон — `auth_test.go`): без cookie `GET` и `PUT` → 401; сессия есть, настроек нет → 404 с кодом `onboarding_required`; `PUT` корректного тела → 200 и тело ответа совпадает; затем `GET` → 200 те же данные; `PUT` с уровнем `D1`, минутами 241, неизвестным поясом, лишним полем, битым JSON, телом больше лимита (`maxSettingsBody = 4 << 10`) → 400 `invalid_request`, в хранилище ничего не записано; `PUT` с чужим `Origin` → 403.
- [ ] **Step 3: Run** `go test ./internal/httpapi/...` — FAIL.
- [ ] **Step 4: Implement** `GetSettings`/`PutSettings` в `settings.go` методами `*authServer` (или отдельным типом, встроенным в сервер): `http.MaxBytesReader`, `json.Decoder` с `DisallowUnknownFields`, `settings.Validate(…, time.Now())`, `UpsertSettings`; ошибки пишутся через `writeError`; 500 логируется через `slog` без тела запроса.
- [ ] **Step 5: Integration test**: в `integration_test.go` по образцу входа: войти, `GET` → 404, `PUT` → 200, `GET` → 200, `PUT` с целью ниже уровня → 400 (проверяет и CHECK в БД не срабатывает раньше Go).
- [ ] **Step 6: Run** `go vet ./... && go test -race ./... && golangci-lint run` — всё чисто. Живая проверка: `docker compose up -d --wait`, `go run ./cmd/server`, `curl` без cookie → 401.
- [ ] **Step 7: Commit**: «API настроек: GET/PUT /settings».

### Task 4: Клиент, хуки и защитники маршрутов

**Files:**
- Regenerate: `web/src/api/schema.gen.ts` (`npm run gen:api`)
- Modify: `web/src/api/client.ts`, `web/src/app/guards.tsx`, `web/src/app/router.tsx`, `web/src/app/guards.test.tsx`
- Create: `web/src/features/settings/settings.ts`, `web/src/features/settings/settings.test.ts`

**Interfaces:**
- Consumes: схема `Settings` из Task 3; `request`/`ApiError` в `client.ts`; `ErrorCode` уже выводится из схемы (добавится `onboarding_required`).
- Produces:
  - `client.ts`: `getSettings(): Promise<Settings>`, `putSettings(s: Settings): Promise<Settings>` (тип `Settings = components['schemas']['Settings']`).
  - `settings.ts`: `settingsKey = ['settings'] as const`; `fetchSettings(): Promise<Settings | null>` (404 `onboarding_required` → `null`, прочие ошибки пробрасывает); `useSettings()` (`retry: false`); `useSaveSettings()` (мутация, `onSuccess` кладёт ответ в кэш `settingsKey`).
  - `guards.tsx`: `RequireOnboarded` (обёртка для рабочих разделов: загрузка → «Проверяем настройки…»; ошибка запроса → экран ошибки с «Повторить» (редиректа в онбординг нет); `null` → `<Navigate to="/onboarding" replace />`) и `RedirectIfOnboarded` (для `/onboarding`: настройки есть → `/`).

- [ ] **Step 1: Failing tests**: `settings.test.ts` — `fetchSettings` при 404 `onboarding_required` возвращает `null`, при 500 бросает; `guards.test.tsx` — `RequireOnboarded` при `null` редиректит на `/onboarding`, при ошибке 500 показывает «Повторить» и не редиректит, при данных рендерит детей; `RedirectIfOnboarded` наоборот.
- [ ] **Step 2: Run** `cd web && npm run gen:api && npx vitest run src/features/settings src/app` — FAIL до реализации.
- [ ] **Step 3: Implement** по Interfaces. В `router.tsx`: маршрут `/onboarding` (внутри `RequireAuth`, вне `AppShell`, с `RedirectIfOnboarded`), корневой `/` оборачивается в `RequireOnboarded` внутри `RequireAuth`; страницы-заготовки пока `ComingSoonPage`.
- [ ] **Step 4: Run** `npx vitest run && npx tsc -b && npm run lint` — PASS.
- [ ] **Step 5: Commit**: «Фронтенд: клиент настроек и защитники онбординга».

### Task 5: Общая форма настроек со счётчиком

**Files:**
- Create: `web/src/features/settings/SettingsForm.tsx`, `web/src/features/settings/ForecastCounter.tsx`, `web/src/features/settings/SettingsForm.test.tsx`, `web/src/features/settings/levelInfo.ts`
- Modify: `DESIGN.md` (новые компоненты — когда интерфейс готов)

**Interfaces:**
- Consumes: `forecastForward`, `forecastBackward`, `describeDuration` (Task 1); `useSaveSettings` (Task 4); `LEVELS`; `Settings`.
- Produces:
  - `levelInfo.ts`: `LEVEL_INFO: Record<Level, { title: string; hint: string }>` — описания простыми словами по-русски для карточек уровня.
  - `ForecastCounter({ current, target, minutesPerDay, targetDate }: {...})` — показывает «до B2 примерно 6–8 месяцев» и ожидаемую дату; если задана дата цели — строку «нужно около X–Y мин в день» и предупреждение при `unrealistic`; для `reached` — «цель уже достигнута»; `role="status"`, `aria-live="polite"`.
  - `SettingsForm({ initial, mode, onSaved }: { initial: Settings | null; mode: 'onboarding' | 'settings'; onSaved: () => void })`. В режиме `onboarding` три шага (уровень → цель и дата → минуты и пояс) с кнопками «Дальше/Назад», в режиме `settings` одна страница со всеми полями. Часовой пояс по умолчанию — `Intl.DateTimeFormat().resolvedOptions().timeZone`. Пресеты минут 10/15/30/60 и своё число. Если выбранная цель ниже уровня, цель подтягивается до уровня.

- [ ] **Step 1: Failing tests** (`SettingsForm.test.tsx`, API подменён через мок `putSettings`): в онбординге нельзя перейти с шага 1 без выбранного уровня; выбор B1 → цель B2 → 90 мин показывает «6–8 месяцев» в счётчике; выбор уровня выше цели поднимает цель; кнопка «Сохранить» блокируется на время запроса и двойной клик отправляет один запрос; ошибка 400 сервера показывает её сообщение (`role="alert"`) и данные в форме остаются; успех вызывает `onSaved` один раз и показывает Sonner-уведомление в режиме `settings`.
- [ ] **Step 2: Run** `npx vitest run src/features/settings/SettingsForm.test.tsx` — FAIL.
- [ ] **Step 3: Implement** компоненты. Стили и поведение по `DESIGN.md` (токены, `surface`, рамки 1px, фокус); карточки уровня — радиогруппа с нормальным управлением с клавиатуры; текст на «ты». Перед вёрсткой вызвать навык `impeccable` и сверить с `.impeccable/surfaces/web.md`.
- [ ] **Step 4: Run** `npx vitest run && npx tsc -b && npm run lint` — PASS.
- [ ] **Step 5: Commit**: «Форма настроек и счётчик срока».

### Task 6: Страницы «Онбординг» и «Настройки»

**Files:**
- Create: `web/src/features/settings/OnboardingPage.tsx`, `web/src/features/settings/SettingsPage.tsx`, тесты рядом
- Modify: `web/src/app/router.tsx` (подставить страницы; маршрут `settings` внутри `AppShell`), `web/src/features/shell/AppShell.tsx` (пункт «Настройки» в `NAV` или в нижней части панели рядом с выходом, иконка `Settings` из lucide-react), `web/src/features/shell/AppShell.test.tsx`

**Interfaces:**
- Consumes: `SettingsForm`, `useSettings`; `document.title` — «Настройка — Verst» / «Настройки — Verst».
- Produces: `OnboardingPage` (при успехе `navigate('/', { replace: true })`) и `SettingsPage`.

- [ ] **Step 1: Failing tests**: онбординг рендерит форму в режиме `onboarding` и после сохранения переходит на `/`; страница настроек подставляет сохранённые значения; пункт «Настройки» есть в меню и подсвечивается на `/settings`.
- [ ] **Step 2: Run** `npx vitest run` — FAIL; **Step 3: Implement**; **Step 4: Run** `npx vitest run && npx tsc -b && npm run lint` — PASS.
- [ ] **Step 5: Commit**: «Страницы онбординга и настроек».

### Task 7: «Сегодня» на реальных настройках

**Files:**
- Modify: `web/src/features/today/TodayPage.tsx`, `web/src/features/today/TodayPage.test.tsx`, `web/src/features/today/demo.ts`

**Interfaces:**
- Consumes: `useSettings`, `forecastForward`, `describeDuration`.
- Produces: заголовок «Цель: {current} → {target} · {minutes} мин в день» из настроек; строка прогноза «до {target} примерно …» (или «цель достигнута»); из `DEMO_TODAY` удалены поля `current`, `target`, `minutesPerDay` (остальное остаётся демо под плашкой).

- [ ] **Step 1: Failing test**: при настройках B1/B2/90 экран показывает «B1 → B2», «90 мин в день» и «6–8 месяцев»; демо-плашка остаётся.
- [ ] **Step 2–4:** Run → FAIL, implement, Run `npx vitest run && npx tsc -b && npm run lint` — PASS.
- [ ] **Step 5: Commit**: «Сегодня: цель и прогноз из настроек».

### Task 8: Сквозные тесты, проверка в браузере, документы

**Files:**
- Create: `web/e2e/onboarding.spec.ts`
- Modify: `web/e2e/screens.spec.ts` (подмена `**/api/v1/settings` в общей настройке, чтобы старые тесты шли с готовыми настройками), `web/e2e/real-api.spec.ts` (по возможности), `DESIGN.md`, `CLAUDE.md` (статус и команды — только проверенные), память проекта

- [ ] **Step 1: Failing e2e** (`onboarding.spec.ts`, API подменён): вход → `GET /settings` отдаёт 404 `onboarding_required` → открыт онбординг; три шага, счётчик показывает вилку; сохранение (`PUT` подменён) → «Сегодня» с выбранной целью; повторная загрузка с готовыми настройками не показывает онбординг; в настройках смена минут и «Сохранить» → уведомление; ошибка 500 на `GET /settings` показывает «Повторить», а не онбординг.
- [ ] **Step 2: Run** `cd web && npm run e2e` — старые тесты не сломаны, новый проходит; снимки в `web/test-results/screens/` просмотреть глазами (светлая и тёмная тема, окна 1024 и 1440).
- [ ] **Step 3: Проверка с настоящим сервером**: `docker compose up -d --wait`, `go run ./cmd/server`, `npm run dev`; вручную пройти онбординг в Playwright-браузере с реальным пользователем (пароль вводит пользователь сам; в чат и репозиторий не попадает); проверить, что строка появилась в `settings`.
- [ ] **Step 4: Документы**: `DESIGN.md` — компоненты формы и счётчика; `CLAUDE.md` — статус («онбординг и настройки сделаны») без описания несуществующего.
- [ ] **Step 5: Финальная проверка**: `go vet ./... && go test -race ./... && golangci-lint run` и `cd web && npx vitest run && npx tsc -b && npm run lint && npm run build`. Затем запустить `web-design-guidelines` по новым файлам и (по согласованию) агента `reviewer` один раз на всю задачу.
- [ ] **Step 6: Commit** по просьбе пользователя.
