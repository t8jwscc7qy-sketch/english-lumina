# Вехи и «вспышка успеха»: план реализации

> **Для исполнителя:** используй superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги отмечай `- [ ]`.

**Цель:** честные вехи из журнала событий (серия, часы, ответы, «первое»), блок «Вехи» на «Прогрессе» и «вспышка успеха» в итоге дня.

**Архитектура:** чистые функции в `internal/activity` (серия вперёд, расчёт вех), обработчик `GET /api/v1/milestones` поверх `ListActivityEvents` (окно 365 суток), фронтенд: блок «Вехи» и компонент вспышки в итоге сессии. Новых таблиц, миграций и SQL-запросов нет.

**Стек:** Go, OpenAPI → oapi-codegen, React + TanStack Query, CSS-анимация. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-25-milestones-design.md`.

## Global Constraints

- Новых зависимостей, миграций и SQL-запросов нет. Контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` (oapi-codegen в `~/go/bin`) и `npm run gen:api` в `web/`; генерированное руками не правим. **Генерированные файлы (`internal/api/api.gen.go`, `web/src/api/schema.gen.ts`) коммитятся вместе с контрактом; перед каждым коммитом проверять `git status` (кроме чужого `web/vite.sandbox.config.ts` ничего не должно остаться неотслеженным или изменённым). Для проверки генерации не запускать oapi-codegen с `-o` в чужой файл: он перезаписывает исходный файл.**
- Интерфейс на русском. **Исполнитель фронтенда обязан вызвать навыки `impeccable`, `web-design-guidelines`, `emil-design-eng` через Skill-инструмент** до кода и описать применение; сверка с `DESIGN.md` и его обновление. Движение допустимо только у «вспышки успеха» (один раз, до 600 мс, ease-out, без движения при `prefers-reduced-motion`).
- Все новые тесты сначала RED (вывод падения в отчёте), потом GREEN.
- Правила (значения из спека, дословно): пороги серии 3, 7, 14, 30, 100, 365 дней; часы 1, 5, 10, 25, 50, 100, 250; ответы 100, 500, 1000; «первое»: карточка, упражнение, закрытая ошибка; окно 365 суток; серия по правилам «Сегодня» (минимум 10 минут, один пропуск на 7 суток прощается), для вех потолок серии 365 (не 60); `id`: `streak-7`, `hours-10`, `answers-100`, `first-card`, `first-exercise`, `first-mistake-closed`; `newToday` — вехи с `reachedOn` равным сегодняшним суткам; массивы в ответе не `null`.
- Базу только читать (SELECT/EXPLAIN); живая проверка под входом пользователя, пароль не записывать.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run && gofmt -l .` (пусто); в `web/` — `npx vitest run && npx tsc -b && npm run lint && npm run build`. Коммитить только свои файлы (не `git add -A`).
- Не пушить; коммиты по-русски, в конце `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

## Review Focus

- Веха ровно на пороге достигается (`>=`), на единицу меньше — нет (Task 2).
- Серия: прямой и обратный подсчёт совпадают на любых историях и в любой день окна, в том числе с пропусками, обрывами и «прощённым пропуском до первого занятия» (Task 1).
- Сегодня без минимума не считается пропуском и не обрывает серию (Task 1).
- Пустая история: все вехи не достигнуты, `newToday = []`, массивы не `null` (Task 2, 3).
- Событие около полуночи попадает в свои сутки по часовому поясу (Task 2, 3).
- Несколько вех в один день: порядок «серия, часы, ответы, первое», самая крупная первой (Task 2, 4).
- Чужие события не влияют; 401 (Task 3).
- Вспышка не показывается без `newToday`; при `prefers-reduced-motion` без движения; после перезагрузки в тот же день показывается снова (Task 4).
- Ошибка загрузки вех не ломает ни итог дня, ни «Прогресс» (Task 4).

## Структура файлов

- `internal/activity/streak_series.go`, `streak_series_test.go`, `milestones.go`, `milestones_test.go`.
- `api/openapi.yaml`, `internal/api/api.gen.go`, `web/src/api/schema.gen.ts`; `internal/httpapi/milestones.go`, `milestones_test.go`, интеграционный тест рядом с существующим.
- `web/src/features/milestones/milestones.ts` (+тест), `MilestonesBlock.tsx` (+тест), `MilestoneFlash.tsx` (+тест); `web/src/features/progress/ProgressPage.tsx`, `web/src/features/session/SessionPage.tsx`, `web/src/index.css` (ключевые кадры вспышки), `web/e2e/milestones.spec.ts`, `web/e2e/mock-api.ts`, `DESIGN.md`, `CLAUDE.md`.

---

### Task 1: Серия вперёд и серия с произвольным потолком

**Files:** Create `internal/activity/streak_series.go`, `streak_series_test.go`; Modify `internal/activity/activity.go` (потолок серии параметром).

**Interfaces:**
- Consumes: `Day`, `Day.AddDays`, `Day.ordinal`, `MinimumDayMinutes`, `ComputeStreak`.
- Produces: `func ComputeStreakN(minutes map[Day]time.Duration, today Day, maxDays int) Streak` (то же, что `ComputeStreak`, но потолок и глубина просмотра `maxDays`; `ComputeStreak` вызывает её с `HistoryDays`, поведение не меняется); `func StreakSeries(minutes map[Day]time.Duration, from, to Day) map[Day]int` — длина серии на конец каждых суток от `from` до `to` воспроизведением вперёд: выполненный минимум `+1`; пропущенный день: если прощённого пропуска ещё не было или с него прошло не меньше 7 суток — прощается (серия не меняется, запоминается день), иначе серия обнуляется и запомненный прощённый пропуск сбрасывается; на последний день `to` пропуск не применяется, если минимум не выполнен (день не закончился).

- [ ] **Step 1: тесты (RED).** `TestComputeStreakNMatchesComputeStreak` (при `maxDays = HistoryDays` результаты равны на нескольких случаях из `activity_test.go`); `TestComputeStreakNLongCap` (100 выполненных суток подряд: `ComputeStreakN(..., 365).Days == 100`, а `ComputeStreak` даёт 60); `TestStreakSeriesBasic` (3 выполненных подряд → 1, 2, 3); `TestStreakSeriesFreeze` (пропуск между выполненными прощён, серия продолжается; второй пропуск в 7 суток обнуляет); `TestStreakSeriesTodayNotDone` (последний день без минимума не обнуляет); `TestStreakSeriesMatchesBackward` — **свойство**: детерминированный генератор (`rand.New(rand.NewPCG(seed, 1))`, не меньше 300 историй длиной 5–120 суток, вероятность выполненного дня 0,3–0,95), для КАЖДОГО конца `e` внутри истории `StreakSeries(m, from, e)[e] == ComputeStreakN(m, e, 400).Days`, где `m` — только сутки до `e` включительно.
- [ ] **Step 2:** `go test ./internal/activity -run 'ComputeStreakN|StreakSeries'` падает. **Step 3:** реализовать. Если свойство находит расхождение, канонична обратная `ComputeStreakN` (её видит пользователь): привести прямую функцию к ней и записать правило в журнал как Ruling; не ослаблять тест. **Step 4:** `go vet`, `golangci-lint run`, `gofmt -l .`; `go test -race ./internal/activity`. **Step 5:** коммит `Вехи: серия вперёд`.

### Task 2: Определения вех и расчёт

**Files:** Create `internal/activity/milestones.go`, `milestones_test.go`.

**Interfaces:**
- Consumes: Task 1 `StreakSeries`; `DayMinutes`, `Day`.
- Produces: `type Milestone struct { ID, Kind, Title string; Threshold int; ReachedOn *Day; Value float64 }` (`Value` — текущее значение линейки в единицах порога: дни, часы, ответы; для «первое» 0 или 1); `type MilestoneInput struct { Minutes map[Day]time.Duration; Answers map[Day]int; FirstCard, FirstExercise, FirstMistakeClosed *Day }`; `func ComputeMilestones(in MilestoneInput, today Day) (all []Milestone, newToday []string)` — порядок: `streak`, `hours`, `answers`, `first` по возрастанию порога; `newToday` в том же порядке, но самая крупная первой по правилу «серия, затем часы, затем ответы, затем первое, внутри линейки больший порог первым».

- [ ] **Step 1: тесты (RED).** `TestMilestonesEmpty` (все не достигнуты, `Value` 0, `newToday` пуст, список полный: 6+7+3+3 = 19); `TestMilestoneHoursThreshold` (ровно 1 ч достигнута в те сутки, где накопленные минуты стали ≥ 60; 59 мин — нет); `TestMilestoneAnswersThreshold` (100 ответов за несколько суток: дата — сутки, где сумма достигла 100; 99 — нет); `TestMilestoneStreak` (7 суток подряд с минимумом → `streak-7` достигнута в седьмые сутки, `Value` = текущая серия); `TestMilestoneFirst` (даты первых событий; `nil` — не достигнута); `TestMilestoneNewToday` (несколько вех сегодня — порядок «серия, часы, ответы, первое», больший порог первым); `TestMilestoneIDsAndTitles` (`streak-7` → «Серия 7 дней», `hours-10` → «10 часов занятий», `hours-1` → «1 час занятий», `answers-100` → «100 ответов», `first-card` → «Первая карточка»; склонения «день/дня/дней», «час/часа/часов»).
- [ ] **Step 2:** падают. **Step 3:** реализовать (пороги константами в одном месте). **Step 4:** `go vet`, `golangci-lint run`, `gofmt -l .`. **Step 5:** коммит `Вехи: определения и расчёт`.

### Task 3: Контракт и `GET /milestones`

**Files:** Modify `api/openapi.yaml`, `internal/api/api.gen.go`, `web/src/api/schema.gen.ts`; Create `internal/httpapi/milestones.go`, `milestones_test.go`; Modify интеграционный тест рядом с существующим.

**Interfaces:**
- Consumes: Task 2; `ListActivityEvents`, `GetSettings`, `activity.Location`, `activity.DayMinutes`, `buildProgress`-стиль обработчика (`progress.go`, `overview.go`), окно 365 суток как в `overview.go`.
- Produces: схемы `MilestoneList` {`milestones: Milestone[]`, `newToday: string[]`}, `Milestone` {`id`, `kind`, `title`, `threshold: integer`, `reachedOn?: string`, `progress: {value: number, target: number}`}; `func buildMilestones(events []db.ListActivityEventsRow, now time.Time, tz string) api.MilestoneList`; `func (s *authServer) GetMilestones(w, r)`.

- [ ] **Step 1: тесты (RED).** `TestBuildMilestonesEmpty` (19 вех, все без `reachedOn`, `progress.value` 0, `newToday` — пустой массив, не `null`); `TestBuildMilestonesFirstEvents` (даты первых событий по типам `card.reviewed`, `exercise.answered`, `mistake.closed`); `TestBuildMilestonesHoursAndAnswers` (события за несколько суток дают `hours-1` и `answers-100` в правильные сутки); `TestBuildMilestonesStreak` (серия за несколько суток); `TestBuildMilestonesNewToday` (веха, достигнутая сегодня, попадает в `newToday`; вчерашняя — нет); `TestBuildMilestonesTimezone` (23:30 UTC при Europe/Moscow относится к следующим суткам); `TestMilestonesEndpoint` (без настроек UTC; чужие события не видны; 401); интеграционный на настоящей Postgres во временной схеме.
- [ ] **Step 2:** падают. **Step 3:** сначала OpenAPI, `go generate ./internal/api`, `npm run gen:api`, затем реализация. **Step 4:** `go build ./... && go test -race ./...`, `golangci-lint run`; `git status` после коммита чист. **Step 5:** `EXPLAIN` не нужен (запрос тот же); отметить в отчёте число событий и время расчёта на 20 000 синтетических событий (тест-бенчмарк или замер в тесте, цель ≤ 150 мс). **Step 6:** коммит `Вехи: контракт и обзор`.

### Task 4: Фронтенд: блок «Вехи» и «вспышка успеха»

**Files:** Create `web/src/features/milestones/milestones.ts`, `milestones.test.ts`, `MilestonesBlock.tsx`, `MilestonesBlock.test.tsx`, `MilestoneFlash.tsx`, `MilestoneFlash.test.tsx`; Modify `web/src/api/client.ts`, `web/src/features/progress/ProgressPage.tsx`, `ProgressPage.test.tsx`, `web/src/features/session/SessionPage.tsx`, `SessionPage.test.tsx`, `web/src/index.css`, `DESIGN.md`.

**Interfaces:**
- Consumes: Task 3 типы и `useMilestones` (хук запроса, `staleTime: 0`, добавить в `milestones.ts`); спрайты маскота в `web/src/features/today/sprites.ts`; `XpBar`; `formatDayRu`.
- Produces: `MilestonesBlock({ milestones })`; `MilestoneFlash({ milestones, ids })`; функции `nextPerKind(milestones)` (ближайшая недостигнутая по каждой линейке) и `sortNew(milestones, ids)` (порядок вспышки).

- [ ] **Step 1: тесты Vitest (RED):** `nextPerKind` (по одной на линейку, достигнутые пропускаются; все достигнуты — линейки нет); `sortNew` (серия, часы, ответы, первое; больший порог первым); блок «Вехи»: достигнутые с датой («7 дней подряд · 19 октября» или «Серия 7 дней · 19 октября»), ближайшая с `XpBar` и «до 14 дней: 7 из 14», ничего не достигнуто — подсказка вместо пустоты, недостигнутое без сигнальных цветов; вспышка: при пустых `ids` ничего нет; при одной вехе карточка `role="status"` с названием; при нескольких список, первая крупнее; при `prefers-reduced-motion` у элемента нет класса анимации; итог дня в `SessionPage` показывает вспышку только после загрузки вех и только при `newToday`; ошибка загрузки вех не ломает итог дня и `ProgressPage` (блок «Вехи» скрыт с «Повторить»); 401 → вход.
- [ ] **Step 2:** падают. **Step 3:** реализовать, **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`** и описав применение; вспышка: ключевые кадры в `index.css` (прозрачность и мягкий ореол из токенов, до 600 мс, `cubic-bezier(0.23, 1, 0.32, 1)`), отключаются в `@media (prefers-reduced-motion: reduce)`; `DESIGN.md`: «Вехи» и «Вспышка успеха». **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Вехи: интерфейс`.

### Task 5: Сквозные тесты, запуск, документы

**Files:** Create `web/e2e/milestones.spec.ts`; Modify `web/e2e/mock-api.ts`, `CLAUDE.md`.

- [ ] **Step 1:** подмена `GET /milestones` в `mock-api.ts` (опция: достигнутые вехи, `newToday`); Playwright: «Прогресс» с блоком «Вехи» (достигнутые и ближайшая); итог сессии со вспышкой (одна веха и несколько) и без неё; снимки в светлой и тёмной теме на 1440/1024/390 px без горизонтальной прокрутки; посмотреть 3–4 снимка глазами. **Step 2:** `npm run e2e` целиком проходит (дважды).
- [ ] **Step 3: запуск в настоящем приложении.** Миграций нет: сервер перезапустить на новом коде; вход под логином пользователя (пароль не записывать); открыть «Прогресс», сверить блок «Вехи» с событиями в базе (первые события, ответы, часы); в песочнице `english_pass_sandbox` (пользователь `claude`, пароль в `/tmp/sandbox-pw`) вставить события за несколько суток так, чтобы сегодня достигалась веха, и проверить вспышку в итоге сессии. Настоящую базу не менять.
- [ ] **Step 4:** обновить `CLAUDE.md` (только проверенное и ограничения), полный набор проверок, ревью ветки (агент `reviewer`), находки исправлять тестом, который сначала падает; перед каждым коммитом `git status` без неотслеженных и изменённых файлов. **Step 5:** коммиты.
