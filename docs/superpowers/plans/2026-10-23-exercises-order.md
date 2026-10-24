# Упражнение «порядок слов»: план реализации

> **Для исполнителя:** используй superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги отмечай `- [ ]`.

**Цель:** второй вид упражнений `order`: собрать английское предложение из перемешанных плиток по русскому переводу; рейтинг по теме `skill:order`.

**Архитектура:** поверх части 1. Чистая сборка и проверка в `internal/exercises`, расширение подбора в `GetNextExercise`, проверка в `AnswerExercise` по виду, новая страница-компонент на фронтенде. Те же таблицы, рейтинг и события.

**Стек:** Go, pgx + sqlc, goose, OpenAPI → oapi-codegen, React + TanStack Query. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-23-exercises-order-design.md` (часть 1: `docs/superpowers/specs/2026-10-23-exercises-gap-design.md`, код в `internal/exercises`, `internal/httpapi/exercises*.go`, `web/src/features/exercises/`).

## Global Constraints

- Новых зависимостей нет; drag and drop не делаем (клики + клавиатура).
- SQL только в `queries/`; контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` (oapi-codegen в `~/go/bin`) и `npm run gen:api` в `web/`; генерированное руками не правим.
- Интерфейс на русском, учебный контент на английском; сверка с `DESIGN.md` и его обновление; **исполнитель фронтенда обязан вызвать навыки `impeccable` и `web-design-guidelines` (и `emil-design-eng` для движения) через Skill-инструмент и описать это в отчёте**.
- Все новые тесты сначала RED (вывод падения в отчёте), потом GREEN; тесты, проходящие вхолостую, не принимаются.
- Миграция `00008` обратимая; **к настоящей базе применять только после явного «да» пользователя** (перед этим `pg_dump`); до этого тесты идут во временной схеме. К настоящей базе допустимо только чтение (SELECT, EXPLAIN).
- Допустимая длина: плиток 4–10; `given` по-прежнему 1..200 символов, `key` 1..64.
- Правильный порядок не уходит клиенту до ответа; плитки перемешаны и порядок не совпадает с исходным.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`; в `web/` — `npx vitest run && npx tsc -b && npm run lint && npm run build`.
- Не пушить; коммиты по-русски, в конце `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

## Review Focus

- Предложение с повторяющимися словами («the … the»): любой порядок, дающий допустимую строку, засчитывается; плитки различимы по индексу (тест Task 1 и фронтенда).
- Альтернативы из других токенов (не хватает плиток для ответа) отбрасываются, а не ломают задание (Task 1).
- Предложение, где плитки случайно совпали с исходным порядком: перемешивание повторяется, затем отказ (Task 1).
- Все плитки использованы, но порядок неверный; и попытка проверить, не использовав все плитки (Task 5: `Enter` не отправляет).
- Ответ вида `order` с лишними пробелами, другим регистром первого слова, без точки в конце: засчитывается, как в спеке (Task 1/4).
- Пустой пул кандидатов для `skill:order` → следующая тема, затем 404 `no_exercise` (Task 4).

## Структура файлов

- `internal/exercises/order.go`, `order_test.go` — сборка и проверка.
- `migrations/00008_exercises_order.sql`, `queries/exercises.sql` (+`ListEnglishAlternatives`).
- `api/openapi.yaml`, сгенерированное.
- `internal/httpapi/exercises_pick.go`, `exercises.go`, `exercises_answer.go` (+ тесты, `exercises_fake_test.go`, `auth.go` интерфейс).
- `web/src/features/exercises/OrderExercise.tsx` (+ тест), `ExercisesPage.tsx` (выбор по `kind`), `topics.ts`, `web/e2e/exercises.spec.ts`, `web/e2e/mock-api.ts`, `DESIGN.md`.

---

### Task 1: Сборка и проверка `order` в `internal/exercises`

**Files:** Create `internal/exercises/order.go`, `internal/exercises/order_test.go`

**Interfaces:**
- Produces: `type Order struct { Tiles []string; Answers []string }`; `func BuildOrder(sentence string, alternatives []string, seed int64) (Order, bool)`; `func CheckOrder(answers []string, given string) bool`; `func OrderTopics(level string) []string` (→ `["skill:order", "level:"+level]`, а при пустом `level` только `["skill:order"]`); `const OrderMinTiles = 4`, `OrderMaxTiles = 10`.

- [ ] **Step 1: тесты (RED).** `TestBuildOrderBasic` («I went home yesterday.» → `Tiles` содержит те же 4 токена с первым словом в нижнем регистре кроме `I`, не равны исходному порядку, `Answers[0] == "I went home yesterday."`); `TestBuildOrderLowercasesFirstWord` («Yesterday I went home.» → плитка `yesterday`, `Answers` сохраняет «Yesterday I went home.»); `TestBuildOrderKeepsApostropheAndHyphen` («I'm well-known here today.»); `TestBuildOrderRejectsCapitalized` («Tom went to Boston today.» → false; «I met Tom today.» → false; «I'll go home today.» → true); `TestBuildOrderLength` (3 токена → false, 11 → false, 4 и 10 → true); `TestBuildOrderAlternatives` (альтернатива с теми же токенами и другим порядком попадает в `Answers`; альтернатива с другим составом токенов отбрасывается); `TestBuildOrderDeterministic` (один seed → те же плитки; разные seed дают плитки, но никогда не равные исходному порядку); `TestBuildOrderDuplicateTokens` («The cat saw the dog today.» → true, `CheckOrder` принимает и «the cat saw the dog today.»); `TestBuildOrderLowerChangesLength` («İ am here today now.» → false); `TestCheckOrder` (лишние пробелы и регистр первого слова не мешают: « the cat saw the dog today. » верно; пунктуация приклеена к токену и сравнивается: «I went home yesterday» без точки → false; длина 201 → false; пустая строка → false).
- [ ] **Step 2:** `go test ./internal/exercises -run Order` падает.
- [ ] **Step 3: реализовать** функции выше. Токены — `strings.Fields`; перемешивание `rand.New(rand.NewPCG(seed, seed))`; до 5 попыток, пока порядок плиток (с учётом нижнего регистра первого) не равен ни одному допустимому; все плитки — токены допустимых ответов с точностью до регистра первого слова; альтернативы проверяются как мультимножество токенов (нижний регистр) против исходного.
- [ ] **Step 4:** тесты проходят; `go vet`, `golangci-lint`.
- [ ] **Step 5:** коммит `Упражнения: сборка и проверка «порядка слов»`.

### Task 2: Миграция `00008` и запрос альтернатив

**Files:** Create `migrations/00008_exercises_order.sql`; Modify `queries/exercises.sql`, `internal/testdb/exercises_queries_test.go` (+ up/down/up по образцу); сгенерированное `internal/db/*`.

**Interfaces:**
- Produces (sqlc): `ListEnglishAlternatives(ctx, ListEnglishAlternativesParams{EnID int64, MaxRows int32}) ([]ListEnglishAlternativesRow{ID int64, Text string}, error)` — другие английские предложения (4–10 слов, `lang='en'`), связанные хотя бы с одним русским переводом данного `EnID` через `sentence_pairs`; без самого `EnID`; порядок по `id`; лимит `MaxRows`. Проверить реальное имя в сгенерированном `internal/db`.

- [ ] **Step 1: тесты (RED)** во временной схеме: миграция up/down/up (после Down `kind` принимает только `gap`; Down с существующей строкой `order` падает с понятной ошибкой — в тесте это проверяется); вставка упражнения `kind='order'` проходит после Up, `kind='x'` — отклоняется; `ListEnglishAlternatives`: два английских предложения с общим русским переводом возвращают друг друга, само предложение не возвращается, предложение вне диапазона слов не возвращается, лимит работает.
- [ ] **Step 2:** падают. **Step 3:** миграция (`ALTER TABLE exercises DROP CONSTRAINT <имя>; ADD CONSTRAINT ... CHECK (kind IN ('gap','order'))`; Down возвращает `('gap')`, в комментарии предупреждение удалить строки `order` перед откатом; имя ограничения узнать из `00007`); запрос в `queries/exercises.sql`; `sqlc generate`.
- [ ] **Step 4:** `go test -race ./internal/db/... ./internal/testdb/...`. **Step 5:** `EXPLAIN (ANALYZE)` для `ListEnglishAlternatives` на настоящей базе (только чтение) — цель ≤ 100 мс, результат в отчёт. **Step 6:** коммит `Упражнения: миграция 00008 и запрос альтернатив`. Миграцию к настоящей базе НЕ применять.

### Task 3: Контракт API и хранилище

**Files:** Modify `api/openapi.yaml`, `internal/httpapi/auth.go` (AuthStore), `internal/httpapi/exercises_fake_test.go`; сгенерированное Go и TS.

**Interfaces:**
- Consumes: Task 2 `ListEnglishAlternatives`.
- Produces: `Exercise.kind` enum `[gap, order]`; `Exercise.tiles` (необязательный `[]string`); метод `ListEnglishAlternatives` в `AuthStore` и реализация в фейке (фейк фильтрует по связи и длине, как SQL).

- [ ] **Step 1:** правки openapi (описание: для `order` `prompt` — русский перевод, `choices` пустой, `tiles` перемешаны; `given` для `order` — предложение строкой), генерация Go и TS. **Step 2:** `go build ./... && go vet ./...`; в `web/`: `npx tsc -b && npm run lint`. **Step 3:** тест фейка (связь и длина) RED→GREEN. **Step 4:** коммит `Упражнения: контракт «порядка слов»`.

### Task 4: Подбор `order` в `next` и проверка в `answer`

**Files:** Modify `internal/httpapi/exercises_pick.go`, `exercises.go`, `exercises_answer.go`; Test `internal/httpapi/exercises_test.go`, `exercises_answer_test.go`, `integration_test.go`.

**Interfaces:**
- Consumes: Task 1 `BuildOrder`, `CheckOrder`, `OrderTopics`; Task 3 `ListEnglishAlternatives`.
- Produces: `GET /exercises/next` может вернуть `kind: order` с `tiles`; `POST /exercises/answer` для `kind=order` проверяет `CheckOrder`.

- [ ] **Step 1: тесты (RED), fakeStore:** `TestNextOrderTopic` (рейтингов нет, `skill:order` первым в списке выбираемых тем → ответ `kind: order`, `tiles` перемешаны и не равны исходному порядку, `choices` пустой, в JSON нет `answers`/исходной строки; запись `exercises` с `topics ⊇ {skill:order, level:...}`); `TestNextOrderUsesAlternatives` (альтернативы попали в `answer.answers`); `TestNextOrderNoCandidatesFallsToNextTopic` (по `skill:order` пусто → следующая тема, не 500); `TestNextOrderLevelCap` (предложение уровня выше «уровень пользователя + 1» отбрасывается); `TestNextOrderNoRepeat` (отвеченное предложение не повторяется по теме `skill:order`); `TestAnswerOrderCorrect/Wrong/Alternative` (верный, неверный, альтернативный порядок; рейтинги `skill:order` и `level:` обновлены; событие `exercise.answered` содержит `kind` и `given`); `TestAnswerOrderNormalization` (лишние пробелы, регистр первого слова). Интеграционный на настоящей Postgres во временной схеме: выдача `order` → ответ → повтор по ключу → 409 с другим ключом.
- [ ] **Step 2:** падают. **Step 3:** `skill:order` добавить в выбираемые темы (пока у пользователя нет данных — включается всегда); подбор кандидатов окном `SampleSentences` + уровень по `SentenceLevels` (правила части 1) + предел «уровень пользователя + 1» + `ListEnglishAlternatives` для кандидата + `BuildOrder`, ≤ 30 кандидатов; `payload={prompt: <ru>, translation: <ru>, tiles, topic}`, `answer={"answers":[...]}`; в `AnswerExercise` проверка по `exercises.kind` (`gap` → `Check`, `order` → `CheckOrder`), остальной путь без изменений. Чистую логику вне `httpapi` держать в `internal/exercises`.
- [ ] **Step 4:** `go test -race ./...`, `golangci-lint run`. **Step 5:** замер на настоящей базе (только чтение): число подходящих предложений 4–10 слов по уровням и время подбора; результат в отчёт. **Step 6:** коммит `Упражнения: подбор и проверка «порядка слов»`.

### Task 5: Фронтенд `OrderExercise`

**Files:** Create `web/src/features/exercises/OrderExercise.tsx`, `OrderExercise.test.tsx`; Modify `ExercisesPage.tsx` (выбор компонента по `kind`), `topics.ts` (`skill:order` → «Порядок слов»), `DESIGN.md`.

**Interfaces:**
- Consumes: Task 3 типы (`Exercise.kind`, `tiles`), `useNextExercise`, мутация ответа и ключ идемпотентности из части 1.

- [ ] **Step 1: тесты Vitest (RED):** русский перевод и плитки видны; клик по плитке переносит её в строку ответа, клик по плитке в строке возвращает назад; цифры 1–9 и 0 переносят плитки; `Backspace` возвращает последнюю, `Esc` очищает; `Enter` не отправляет, пока не использованы все плитки, и отправляет `given` = слова через пробел, когда использованы; двойной `Enter` — один запрос; после ответа видны «Верно/Неверно» (не только цветом), правильное предложение целиком, перевод, «Дальше» (Пробел); ошибки ответа 409/404/5xx ведут себя как в части 1 («Повторить»/«Пропустить»); 401 → вход; повторяющиеся плитки (два `the`) выбираются независимо; `lang="en"` у плиток; живой регион результата; доступные имена плиток и `aria-keyshortcuts`; в DOM до ответа нет правильного порядка (плитки в порядке сервера).
- [ ] **Step 2:** падают. **Step 3:** реализовать по образцу `ExercisesPage.tsx`/`GapExercise` (фокус на контейнере, `e.repeat`, `busyRef`), **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`** и описав применение в отчёте. Контраст значков ≥ 3:1 (токен `--grass-text`). `DESIGN.md`: плитка слова, строка ответа, состояния.
- [ ] **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Упражнения: интерфейс «порядка слов»`.

### Task 6: Сквозные тесты, запуск, документы

**Files:** Modify `web/e2e/mock-api.ts`, `web/e2e/exercises.spec.ts`, `CLAUDE.md`; Create отчёт.

- [ ] **Step 1:** подмена `order` в `mock-api.ts` (очередь фикстур с `kind: order`, проверка ответа по допустимым строкам); Playwright: собрать предложение клавишами → «Верно» → Пробел; неверный порядок → показан правильный; повторяющиеся плитки; снимки в светлой и тёмной теме на 1440/1024/390 px без горизонтальной прокрутки. **Step 2:** `npm run e2e` целиком проходит.
- [ ] **Step 3: запуск в настоящем приложении.** Спросить у пользователя «да» на применение `00008` (перед этим `pg_dump`, как для `00007`); применить; сервер и фронтенд поднять; вход в приложение выполняет **пользователь** (создавать сессию в базе в обход входа нельзя); после входа пройти «порядок слов» в окне Playwright, сверить в базе строки `exercises` вида `order`, `topic_ratings` для `skill:order`, `events`. Если пользователь вход не даёт — не заявлять проверку в живом приложении.
- [ ] **Step 4:** обновить `CLAUDE.md` (только проверенное, ограничения: порядки вне найденных альтернатив засчитываются ошибкой, пунктуация приклеена к плиткам), полный набор проверок, запустить ревью ветки и исправить находки тестом, который сначала падает.
- [ ] **Step 5:** коммиты.
