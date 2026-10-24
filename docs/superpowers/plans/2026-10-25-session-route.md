# Маршрут дня: план реализации

> **Для исполнителя:** используй superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги отмечай `- [ ]`.

**Цель:** честные минуты, «минимальный день» и серия из журнала событий; кнопка «Начать сессию» ведёт на страницу `/session` (карточки → упражнения → итог дня).

**Архитектура:** поверх прежних частей. Чистые функции `activity.Minutes` и `activity.Streak` в новом пакете `internal/activity`; один запрос событий за 60 суток и новый путь `GET /api/v1/progress/today`; фронтенд: выделение встраиваемых `ReviewCard` и `ExerciseCard`, «Сегодня» с настоящими данными, страница `/session`. Новых таблиц и миграций нет.

**Стек:** Go, pgx + sqlc, OpenAPI → oapi-codegen, React + TanStack Query. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-25-session-route-design.md`.

## Global Constraints

- Новых зависимостей и миграций нет. SQL только в `queries/`; контракт сначала в `api/openapi.yaml`, затем `sqlc generate`, `go generate ./internal/api` (oapi-codegen в `~/go/bin`), `npm run gen:api` в `web/`; генерированное руками не правим.
- Интерфейс на русском. **Исполнитель фронтенда обязан вызвать навыки `impeccable`, `web-design-guidelines`, `emil-design-eng` через Skill-инструмент** и описать применение в отчёте; сверка с `DESIGN.md` и его обновление.
- Все новые тесты сначала RED (вывод падения в отчёте), потом GREEN.
- Правила (значения из спека, дословно): минуты — пауза между ответами до 60 с засчитывается полностью, иначе и для первого ответа дня +20 с; `MinimumDayMinutes = 10`; сутки по часовому поясу из настроек (по умолчанию UTC, `mistakes.DayStart`); серия — подряд идущие сутки с выполненным минимумом назад от вчера, сегодня добавляется, если минимум уже выполнен; заморозка — один пропуск прощается, если в предыдущие 6 суток не было другого прощённого; второй пропуск в окне 7 суток обрывает серию; история 60 суток; лимит 10 карточек за сессию; «Закончить» доступно всегда; `goalMinutes` из настроек, без настроек 15.
- Учитываются только события `card.reviewed` и `exercise.answered` (минуты); `mistake.opened`/`mistake.closed` — только счётчики.
- Базу только читать (SELECT/EXPLAIN), записей и миграций нет; живая проверка под входом пользователя, пароль не записывать.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run && gofmt -l .` (пусто); в `web/` — `npx vitest run && npx tsc -b && npm run lint && npm run build`. Коммитить только свои файлы (не `git add -A`; `web/vite.sandbox.config.ts` не твой).
- Не пушить; коммиты по-русски, в конце `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

## Review Focus

- События двух суток подряд около полуночи не склеиваются в одну паузу (Task 1, 3).
- День без единого ответа и день с одним ответом (20 с) не ломают серию и минуты (Task 1).
- Серия: два пропуска в окне 7 суток обрывают, один прощается и не прощается второй раз подряд (Task 1).
- Пользователь без настроек: UTC и `goalMinutes = 15`, не 500 (Task 3).
- Чужие события не попадают в минуты и серию (Task 3).
- Очередь карточек пуста: шаг «Карточки» пропускается без пустого экрана; упражнений нет вообще — понятное сообщение, а не зацикливание (Task 5).
- «Закончить» посреди карточки/упражнения не теряет уже данные ответы (Task 5).
- Минуты 9,9 и 10,0 на границе: «Продолжить сессию» против «Позаниматься ещё» (Task 4).

## Структура файлов

- `internal/activity/activity.go`, `activity_test.go`.
- `queries/events.sql` (+`ListActivityEvents`), `api/openapi.yaml`, сгенерированное, `web/src/api/schema.gen.ts`, `internal/httpapi/auth.go` (интерфейс хранилища) и фейки.
- `internal/httpapi/progress.go`, `progress_test.go`, интеграционный тест рядом с существующим.
- `web/src/features/cards/ReviewCard.tsx` (+тест), `web/src/features/exercises/ExerciseCard.tsx` (+тест) — вынесены из страниц; `CardsPage.tsx`, `ExercisesPage.tsx` их используют.
- `web/src/features/today/TodayPage.tsx` (+тест), `web/src/features/today/progress.ts` (хук `useProgressToday`), `web/src/features/session/SessionPage.tsx` (+тест), `web/src/app/router.tsx`, `web/e2e/session.spec.ts`, `web/e2e/mock-api.ts`, `DESIGN.md`, `CLAUDE.md`.

---

### Task 1: Минуты и серия (чистые функции)

**Files:** Create `internal/activity/activity.go`, `activity_test.go`.

**Interfaces:**
- Produces: `package activity`: `const MinimumDayMinutes = 10`, `PauseCap = 60 * time.Second`, `BurstStart = 20 * time.Second`, `HistoryDays = 60`; `type Day struct{ Y, M, D int }` с `func DayOf(t time.Time, tz string) Day`, `func (d Day) AddDays(n int) Day`; `func Minutes(times []time.Time) time.Duration` (времена одного суток-интервала, любой порядок; пустой список → 0); `func DayMinutes(times []time.Time, tz string) map[Day]time.Duration` (группирует по суткам в поясе и считает `Minutes` на каждые сутки отдельно); `type Streak struct{ Days int; FreezeUsedOn *Day }`; `func ComputeStreak(minutes map[Day]time.Duration, today Day) Streak`.

- [ ] **Step 1: тесты (RED).** `TestMinutesEmpty` (0); `TestMinutesSingleAnswer` (20 с); `TestMinutesPauseCap` (ответы через 30 с и 60 с → 20 + 30 + 60 = 110 с; через 61 с → 20 + 20 = 40 с); `TestMinutesUnordered` (порядок на вход не важен); `TestDayMinutesMidnightNotMerged` (ответы 23:59:50 и 00:00:10 Europe/Moscow при паузе 20 с попадают в разные сутки, каждые сутки получают по 20 с); `TestDayOfTimezone` (23:30 UTC 2026-10-24 в Europe/Moscow — 2026-10-25; неизвестный пояс → UTC); `TestStreakNoData` (0); `TestStreakThreeDays` (вчера, позавчера, три дня назад выполнены → 3, сегодня без минимума серию не обнуляет); `TestStreakTodayAdded` (сегодня выполнено + вчера → 2); `TestStreakFreezeOneMiss` (пропущен вчера, выполнены позавчера и день до → серия не обрывается, `FreezeUsedOn` = вчера); `TestStreakTwoMissesBreak` (пропуски вчера и позавчера → 0); `TestStreakSecondFreezeWithinWeekBreaks` (пропуск 2 дня назад, прощён; ещё пропуск 5 дней назад внутри окна 7 суток → обрыв там); `TestStreakFreezeRefillsAfterWeek` (пропуски на расстоянии 8 суток оба прощаются); `TestStreakCap60`.
- [ ] **Step 2:** `go test ./internal/activity` падает. **Step 3:** реализовать (`Day` без зависимости от `time.Location` вне `DayOf`). **Step 4:** тесты, `go vet`, `golangci-lint`, `gofmt -l .`. **Step 5:** коммит `Маршрут дня: минуты и серия`.

### Task 2: Запрос событий и контракт

**Files:** Modify `queries/events.sql`, `api/openapi.yaml`, `internal/httpapi/auth.go` (`AuthStore`), фейки (`auth_test.go`, `exercises_fake_test.go`); регенерация `internal/db`, `internal/api`, `web/src/api/schema.gen.ts`; Create/Modify `internal/testdb/activity_test.go`.

**Interfaces:**
- Produces: SQL `ListActivityEvents(user_id, since)` → `{ Type, OccurredAt, Correct }` для событий `card.reviewed`, `exercise.answered`, `mistake.opened`, `mistake.closed` с `occurred_at >= since` (одна выборка, порядок по `occurred_at, id`, `Correct` из `payload->>'correct'` как nullable boolean); метод в интерфейсе и фейке (фейк хранит срез событий). OpenAPI: `GET /api/v1/progress/today` и схемы `ProgressToday`, `TodayCounts` по спеку (`minutes` — number, `minimumDay` — integer, `goalMinutes`, `doneToday`, `streakDays`, `freezeUsedOn` — date nullable, `today` с `answers`, `correct`, `cardsReviewed`, `mistakesOpened`, `mistakesClosed`).

- [ ] **Step 1: тесты (RED), временная схема:** `ListActivityEvents` возвращает только четыре типа, только нужного пользователя (в тесте снять `users_single_row` во временной схеме), не раньше `since`, в порядке времени; `Correct` пуст у событий без поля; `EXPLAIN` использует индекс `events_user_type_occurred` или `events_user_occurred`.
- [ ] **Step 2:** падают. **Step 3:** запрос, OpenAPI, `sqlc generate`, `go generate ./internal/api`, `npm run gen:api`, фейки. **Step 4:** `go build ./... && go test -race ./internal/testdb/... ./internal/httpapi/...`; `npx tsc -b` в `web/`. **Step 5:** `EXPLAIN (ANALYZE)` на настоящей базе (только чтение), цель ≤ 20 мс, в отчёт. **Step 6:** коммит `Маршрут дня: запрос событий и контракт`.

### Task 3: Обработчик `GET /progress/today`

**Files:** Create `internal/httpapi/progress.go`, `progress_test.go`; Modify интеграционный тест рядом с существующим.

**Interfaces:**
- Consumes: Task 1 `activity.*`, `mistakes.DayStart`; Task 2 `ListActivityEvents`, типы API, `GetSettings`.
- Produces: `func (s *authServer) GetProgressToday(w, r)`; чистая функция сборки ответа `buildProgress(events []db.ListActivityEventsRow, now time.Time, tz string, goal int) api.ProgressToday` (без обращения к базе, для теста).

- [ ] **Step 1: тесты (RED):** `TestProgressEmpty` (нули, `doneToday=false`, `goalMinutes=15` без настроек, `freezeUsedOn` нет); `TestProgressMinutesAndCounts` (набор событий за сегодня → минуты, `answers`, `correct`, `cardsReviewed`, `mistakesOpened`, `mistakesClosed`); `TestProgressDoneToday` (9,9 и 10,0 минут: граница); `TestProgressStreak` (серия по предыдущим суткам, заморозка в ответе); `TestProgressTimezone` (сутки по поясу настроек); `TestProgressOtherUserHidden`; `TestProgressUnauthorized` (401). Интеграционный на настоящей Postgres во временной схеме: события за три дня → минуты и серия.
- [ ] **Step 2:** падают. **Step 3:** реализовать; окно запроса `since = начало суток 60 дней назад`; `goalMinutes` из настроек (`daily_min_minutes`), 15 без них. **Step 4:** `go test -race ./...`, `golangci-lint run`. **Step 5:** коммит `Маршрут дня: прогресс за сегодня`.

### Task 4: «Сегодня» с настоящими данными и встраиваемые компоненты

**Files:** Create `web/src/features/cards/ReviewCard.tsx`, `ReviewCard.test.tsx`, `web/src/features/exercises/ExerciseCard.tsx`, `ExerciseCard.test.tsx`, `web/src/features/today/progress.ts`; Modify `CardsPage.tsx`, `ExercisesPage.tsx`, `TodayPage.tsx`, `TodayPage.test.tsx`, `web/src/features/today/demo.ts`, `DESIGN.md`.

**Interfaces:**
- Consumes: Task 2 типы `ProgressToday`.
- Produces: `useProgressToday()` (запрос `GET /api/v1/progress/today`, `staleTime: 0`); `ReviewCard({ card, onRated })` и `ExerciseCard({ exercise, onNext, loadingNext, onAnswered })` — те же компоненты, что были внутри страниц, с колбэками «оценена карточка» и «ответ получен»; `sessionButtonLabel(p: ProgressToday): 'Начать сессию' | 'Продолжить сессию' | 'Позаниматься ещё'`.

- [ ] **Step 1: тесты Vitest (RED):** страницы `/cards` и `/exercises` ведут себя как раньше (существующие тесты не меняются и проходят); `ReviewCard` и `ExerciseCard` вызывают колбэки ровно один раз после успешной оценки или ответа; «Сегодня» показывает серию `M дней`, «N из 10 мин», пометку заморозки при `freezeUsedOn`; `sessionButtonLabel` для 0, 9,9 и 10,0 минут; кнопка активна и ведёт на `/session`; демо-плашка остаётся только у блоков пути и прогноза; ошибка загрузки прогресса: сообщение и «Повторить», страница остаётся рабочей.
- [ ] **Step 2:** падают. **Step 3:** вынести компоненты без изменения поведения (диф страниц должен быть почти чисто перенос), подключить хук, **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`**; `DESIGN.md`: «Сегодня» с настоящими данными. **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Маршрут дня: «Сегодня» с настоящими данными`.

### Task 5: Страница `/session`

**Files:** Create `web/src/features/session/SessionPage.tsx`, `SessionPage.test.tsx`; Modify `web/src/app/router.tsx`, `DESIGN.md`.

**Interfaces:**
- Consumes: Task 4 `ReviewCard`, `ExerciseCard`, `useProgressToday`, `sessionButtonLabel`; существующие `useCardQueue`, `useNextExercise`.
- Produces: маршрут `/session`; константа `MAX_CARDS_PER_SESSION = 10`.

- [ ] **Step 1: тесты Vitest (RED):** нет карточек на сегодня → сразу упражнения, шага «Карточки» нет; карточки есть → по одной со счётчиком «Карточка 3 из 10», после десятой переход к упражнениям; упражнения идут, пока `minutes < 10`; при `minutes >= 10` экран «Минимум на сегодня выполнен» с «Закончить» и «Ещё немного»; «Закончить» в любой момент ведёт на итог дня и не теряет ответы; итог показывает минуты, ответы и долю верных, ошибки открыто и закрыто, серию и заморозку; упражнений нет вообще (404 `no_exercise`) → сообщение и «На главную», без зацикливания; 401 → вход; ошибки загрузки: сообщение и «Повторить»; полоса из 10 сегментов обновляется после ответа.
- [ ] **Step 2:** падают. **Step 3:** реализовать по образцу существующих страниц, **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`** и описав применение в отчёте; фокус на карточке шага при каждом новом шаге; `DESIGN.md`: раздел «Сессия». **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Маршрут дня: страница сессии`.

### Task 6: Сквозные тесты, запуск, документы

**Files:** Create `web/e2e/session.spec.ts`; Modify `web/e2e/mock-api.ts`, `CLAUDE.md`.

- [ ] **Step 1:** подмена `GET /progress/today` (минуты растут после каждого ответа в подмене) и очереди карточек; Playwright: «Сегодня» с кнопкой в трёх состояниях; сессия целиком (карточки → упражнения → «Минимум выполнен» → итог); «Закончить» посреди; сессия без карточек; снимки «Сегодня», шага и итога в светлой и тёмной теме на 1440/1024/390 px без горизонтальной прокрутки; посмотреть 3–4 снимка глазами и записать замечания. **Step 2:** `npm run e2e` целиком проходит (дважды, без нестабильности).
- [ ] **Step 3: запуск в настоящем приложении.** Миграций нет: сервер перезапустить на новом коде; вход под логином пользователя (если он его даёт, через Chromium проекта, пароль не записывать); открыть «Сегодня», проверить, что минуты и серия совпадают с событиями в базе (SELECT), пройти несколько ответов в `/session`, сверить, как выросли минуты; в `english_pass_sandbox` проверить серию и заморозку на событиях за несколько суток (вставка событий только в песочницу). Не крутить страницу «вслепую». Без входа не заявлять проверку вживую.
- [ ] **Step 4:** обновить `CLAUDE.md` (только проверенное и ограничения), полный набор проверок, ревью ветки (агент `reviewer`), находки исправлять тестом, который сначала падает. **Step 5:** коммиты.
