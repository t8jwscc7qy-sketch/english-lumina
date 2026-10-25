# Страница «Прогресс»: план реализации

> **Для исполнителя:** используй superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги отмечай `- [ ]`.

**Цель:** страница `/progress`: часы занятий до цели, прогноз срока по реальному темпу («собираю данные» первые 14 суток), темп, карта тем.

**Архитектура:** сервер отдаёт только факты (`GET /api/v1/progress/overview` из журнала событий, чистые `activity.Pace` и `activity.CumulativeSeries`); прогноз считает фронтенд чистой функцией рядом с `forecast.ts`; график рисуется SVG вручную. Новых таблиц, миграций и запросов нет (используется `ListActivityEvents`).

**Стек:** Go, OpenAPI → oapi-codegen, React + TanStack Query. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-25-progress-page-design.md`.

## Global Constraints

- Новых зависимостей, миграций и SQL-запросов нет. Контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` (oapi-codegen в `~/go/bin`) и `npm run gen:api` в `web/`; генерированное руками не правим. **Генерированные файлы (`internal/api/api.gen.go`, `web/src/api/schema.gen.ts`) коммитятся вместе с контрактом; перед каждым коммитом проверять `git status`: ничего не должно остаться неотслеженным или изменённым.**
- Интерфейс на русском. **Исполнитель фронтенда обязан вызвать навыки `impeccable`, `web-design-guidelines`, `emil-design-eng` (и `dataviz` для графика) через Skill-инструмент** и описать применение в отчёте; сверка с `DESIGN.md` и его обновление. Движения нет.
- Все новые тесты сначала RED (вывод падения в отчёте), потом GREEN.
- Правила (значения из спека, дословно): окно 365 суток; `daily` — последние 90 суток без пропусков; темп — среднее за 14 суток, включая пустые; `paceReady = daysSinceStart >= 14`; нужные часы — диапазон `forecast.ts` (Cambridge × 1,5–2), без изменений; остаток = `max(0, нужные − набранные)` отдельно по границам; «что изменить» — темп +10 минут в день, показывается при экономии ≥ 1 недели и пока темп ≤ 240 минут в день; максимум 240 минут в день как в `forecast.ts`.
- Тексты из базы выводятся только текстом; пояс — часовой пояс настроек (без настроек UTC), разбирается один раз на запрос (`activity.Location`).
- Базу только читать (SELECT/EXPLAIN); живая проверка под входом пользователя, пароль не записывать.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run && gofmt -l .` (пусто); в `web/` — `npx vitest run && npx tsc -b && npm run lint && npm run build`. Коммитить только свои файлы (не `git add -A`; `web/vite.sandbox.config.ts` не твой).
- Не пушить; коммиты по-русски, в конце `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

## Review Focus

- Нет занятий вообще: `startedOn` нет, `daily` из 90 нулей, прогноз «собираю данные», страница не падает (Task 1, 2, 4).
- Ровно 14 суток: `paceReady` включается в сутки 14, а не 15 (Task 1, 2).
- Сутки с одним ответом и сутки без ответов в середине истории: накопленные часы монотонны (Task 1).
- Остаток часов 0 и «между границами» (Task 3).
- Темп 0 при `paceReady` — «занятий нет», а не деление на ноль и не бесконечная дата (Task 3).
- Темп выше 240 минут в день: нет подсказки «+10» и нет выдуманной даты (Task 3).
- Часовой пояс: событие около полуночи попадает в свои сутки (Task 2).
- Чужие события не видны; 401 (Task 2).
- Часы и минуты по-русски («14 ч 20 мин», «0 мин»), числа не прыгают при ширине 390 px (Task 4).

## Структура файлов

- `internal/activity/overview.go`, `overview_test.go` — `Pace`, `CumulativeSeries`, `StartedOn`.
- `api/openapi.yaml`, `internal/api/api.gen.go`, `web/src/api/schema.gen.ts`; `internal/httpapi/overview.go`, `overview_test.go`, интеграционный тест рядом с существующим.
- `web/src/lib/progressForecast.ts`, `progressForecast.test.ts`; `web/src/features/progress/progress.ts` (хук, форматирование) и тест, `ProgressPage.tsx`, `ProgressPage.test.tsx`, `HoursChart.tsx` (+тест); `web/src/app/router.tsx`, `web/e2e/progress.spec.ts`, `web/e2e/mock-api.ts`, `DESIGN.md`, `CLAUDE.md`.

---

### Task 1: Темп и накопленные часы (чистые функции)

**Files:** Create `internal/activity/overview.go`, `overview_test.go`.

**Interfaces:**
- Consumes: `activity.Day`, `Day.AddDays`, `DayMinutes` из `internal/activity`.
- Produces: `type DayPoint struct { Day Day; Minutes time.Duration; CumulativeHours float64 }`; `func Pace(perDay map[Day]time.Duration, today Day, days int) time.Duration` (среднее на сутки за `days` суток от `today−days+1` до `today`, включая пустые; `days <= 0` → 0); `func CumulativeSeries(perDay map[Day]time.Duration, today Day, days int) []DayPoint` (ровно `days` точек от `today−days+1` до `today`, накопленные часы с начала всей `perDay`, а не с начала окна — суммируются и более ранние сутки); `func StartedOn(perDay map[Day]time.Duration) (Day, bool)` (самые ранние сутки с ненулевыми минутами; нет таких — `false`); `func DaysSince(start, today Day) int` (число суток от `start` до `today` включительно: сегодня = 1).

- [ ] **Step 1: тесты (RED).** `TestPaceEmpty` (0); `TestPaceFullWindow` (14 суток по 10 минут → 10 минут); `TestPaceCountsEmptyDays` (7 суток из 14 по 20 минут → 10 минут); `TestPaceIgnoresOutsideWindow` (сутки на 15 суток назад не считаются); `TestPaceNonPositiveDays` (0); `TestCumulativeSeriesLength` (90 точек, первая `today−89`, последняя `today`); `TestCumulativeSeriesMonotonic` (пустые сутки между занятыми не уменьшают значение; значения в часах: 90 минут → 1,5); `TestCumulativeSeriesIncludesEarlier` (занятия до окна входят в накопленные часы первой точки); `TestStartedOn` (самые ранние ненулевые сутки; нулевые и пустые не считаются; нет занятий → false); `TestDaysSince` (сегодня = 1; вчера = 2; граница месяца).
- [ ] **Step 2:** `go test ./internal/activity -run 'Pace|Cumulative|StartedOn|DaysSince'` падает. **Step 3:** реализовать. **Step 4:** `go vet`, `golangci-lint run`, `gofmt -l .`. **Step 5:** коммит `Прогресс: темп и накопленные часы`.

### Task 2: Контракт и `GET /progress/overview`

**Files:** Modify `api/openapi.yaml`, `internal/api/api.gen.go`, `web/src/api/schema.gen.ts`, `internal/httpapi/progress.go` (или создать `overview.go`); Create `overview_test.go`; Modify интеграционный тест рядом с существующим.

**Interfaces:**
- Consumes: Task 1 функции; `ListActivityEvents`, `GetSettings`, `buildProgress`-стиль обработчика из `progress.go`; `activity.Location`, `activity.DayMinutes`, `mistakes.DayStart`.
- Produces: схема `ProgressOverview` {`startedOn?: string`, `daysSinceStart: integer`, `studiedMinutes: number`, `paceMinutesPerDay: number`, `paceReady: boolean`, `daily: DailyPoint[]`}, `DailyPoint` {`day: string`, `minutes: number`, `cumulativeHours: number`}; `func buildOverview(events []db.ListActivityEventsRow, now time.Time, tz string) api.ProgressOverview` (чистая, без базы); `func (s *authServer) GetProgressOverview(w, r)`; окно `since` — начало суток 365 суток назад в поясе пользователя (`today.AddDays(-365)` и `mistakes.DayStart`).

- [ ] **Step 1: тесты (RED).** `TestBuildOverviewEmpty` (нет `startedOn`, `daysSinceStart` 0, `studiedMinutes` 0, `paceReady` false, `daily` из 90 записей с нулями, не `null`); `TestBuildOverviewPaceReadyBoundary` (первое занятие было `today−13` → `daysSinceStart` = 14 и `paceReady` = true; `today−12` → 13 и false); `TestBuildOverviewMinutesAndDaily` (события за три дня → `studiedMinutes`, значения `daily`, накопленные часы монотонны, десятые округлены); `TestBuildOverviewTimezone` (событие 23:30 UTC при Europe/Moscow относится к следующим суткам); `TestOverviewEndpoint` (без настроек UTC; с настройками пояс; чужие события не видны; 401); интеграционный на настоящей Postgres во временной схеме (события за несколько суток → `startedOn`, `daysSinceStart`, `daily`).
- [ ] **Step 2:** падают. **Step 3:** сначала OpenAPI, `go generate ./internal/api`, `npm run gen:api`, затем реализация. **Step 4:** `go build ./... && go test -race ./...`, `golangci-lint run`; `git status` чистый после коммита (генерированные файлы внутри коммита). **Step 5:** замер на настоящей базе (только чтение, `BEGIN READ ONLY`): `EXPLAIN (ANALYZE)` запроса событий за 365 суток, цель ≤ 50 мс, результат в отчёт. **Step 6:** коммит `Прогресс: контракт и обзор для страницы`.

### Task 3: Прогноз по темпу и форматирование (чистые функции фронтенда)

**Files:** Create `web/src/lib/progressForecast.ts`, `progressForecast.test.ts`, `web/src/features/progress/progress.ts`, `progress.test.ts`; Modify `web/src/api/client.ts` (`getProgressOverview`, тип `ProgressOverview`).

**Interfaces:**
- Consumes: `LEVELS`, `forecast.ts` (экспорт `baseHours`/константы нужных часов: если сейчас они не экспортируются, экспортировать минимально без смены значений и поведения) и `ProgressOverview` из схемы Task 2.
- Produces: `type PaceForecast = { kind: 'reached' } | { kind: 'enough' } | { kind: 'collecting'; daysSinceStart: number } | { kind: 'idle' } | { kind: 'estimate'; needed: Range; remaining: Range; days: Range; finishBy: { earliest: Date; latest: Date }; saveWithTenMore?: { weeks: number }; unrealistic: boolean }`; `function forecastFromPace(current: Level, target: Level, studiedMinutes: number, paceMinutesPerDay: number, paceReady: boolean, daysSinceStart: number, today: Date): PaceForecast`; `useProgressOverview()` (запрос, `staleTime: 0`); `formatHoursMinutes(minutes: number): string` («14 ч 20 мин», «45 мин», «0 мин»); `formatRange(a: number, b: number): string` для часов («280–370 ч»).

- [ ] **Step 1: тесты Vitest (RED):** `reached` при цели не выше уровня; `collecting` при `paceReady=false` (с `daysSinceStart`); `idle` при `paceReady` и темпе 0; `estimate` B1→B2 при 30 минутах в день и 0 набранных: диапазон срока совпадает с `forecastForward(B1, B2, 30, today)`; вычитание набранных часов отдельно по границам (например, набрано столько, сколько нижняя граница нужных: нижний остаток 0, верхний больше 0 → срок от 0 суток); `enough` при набранном не меньше верхней границы; `saveWithTenMore.weeks` ≥ 1 появляется и считается как разница верхних границ сроков, округление до целых; при экономии меньше недели поля нет; при темпе больше 240 `unrealistic: true` и поля `saveWithTenMore` нет; `formatHoursMinutes` (0, 45, 60, 860 → «14 ч 20 мин»); `formatRange`.
- [ ] **Step 2:** падают. **Step 3:** реализовать. **Step 4:** `npx vitest run && npx tsc -b && npm run lint`. **Step 5:** коммит `Прогресс: прогноз по темпу`.

### Task 4: Страница `/progress`

**Files:** Create `web/src/features/progress/ProgressPage.tsx`, `ProgressPage.test.tsx`, `HoursChart.tsx`, `HoursChart.test.tsx`; Modify `web/src/app/router.tsx`, `DESIGN.md`.

**Interfaces:**
- Consumes: Task 3 `forecastFromPace`, `useProgressOverview`, форматирование; `useSettings`, `useTopics` и `topicName`/`reliabilityLabel` из `features/exercises`.
- Produces: маршрут `/progress`; `HoursChart({ points: { day: string; cumulativeHours: number }[]; neededMin: number; neededMax: number })`.

- [ ] **Step 1: тесты Vitest (RED):** заголовок «Прогресс»; нет занятий — «Собираю данные», счётчик «0 из 14 суток», в графике текст «Начни сессию, и здесь появится линия»; оценка — фраза «До B2 примерно …» и даты, «Что изменить» с экономией; «собираю данные» с `daysSinceStart` 5; «Сейчас занятий нет» при `idle`; `enough` без даты; полоса «Набрано … из …»; график для читалок скрыт, таблица в `details` «Данные по дням»; «Темп»: «Средний темп за 14 дней» при `paceReady`, иначе «Темп покажу после 14 суток занятий»; карта тем отсортирована по рейтингу по возрастанию, у пяти самых слабых «слабая тема» текстом; ошибка загрузки — «Повторить»; 401 → вход; `HoursChart` строит ровно `points.length` точек линии и подписи осей; пустой список — без падения.
- [ ] **Step 2:** падают. **Step 3:** реализовать по образцу существующих страниц, **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`, `dataviz`**; цвета только из токенов, `tabular-nums`; `DESIGN.md`: раздел «Прогресс». **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Прогресс: страница`.

### Task 5: Сквозные тесты, запуск, документы

**Files:** Create `web/e2e/progress.spec.ts`; Modify `web/e2e/mock-api.ts`, `CLAUDE.md`.

- [ ] **Step 1:** подмена `GET /progress/overview` в `mock-api.ts` (опция с фикстурой: нет занятий, 5 суток, 20 суток); Playwright: страница пустая, «собираю данные», оценка с подсказкой, карта тем; переход из меню «Прогресс»; снимки страницы и графика в светлой и тёмной теме на 1440/1024/390 px без горизонтальной прокрутки; посмотреть 3–4 снимка глазами. **Step 2:** `npm run e2e` целиком проходит (дважды).
- [ ] **Step 3: запуск в настоящем приложении.** Миграций нет: сервер перезапустить на новом коде; вход под логином пользователя (если он его даёт, через Chromium проекта, пароль не записывать); открыть `/progress`, сверить числа с событиями в базе (SELECT): `startedOn`, `studiedMinutes`, `daily`; у пользователя занятий мало — ожидается «Собираю данные». Для проверки «оценка с подсказкой» вставить события за 20 суток только в песочницу `english_pass_sandbox` (отдельный пользователь `claude`, пароль в `/tmp/sandbox-pw`), настоящую базу не менять. Не крутить страницу «вслепую».
- [ ] **Step 4:** обновить `CLAUDE.md` (только проверенное и ограничения), полный набор проверок, ревью ветки (агент `reviewer`), находки исправлять тестом, который сначала падает; перед каждым коммитом `git status` без неотслеженных и изменённых файлов. **Step 5:** коммиты.
