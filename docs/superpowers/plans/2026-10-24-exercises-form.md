# Упражнение «выбор формы» и рамка умений: план реализации

> **Для исполнителя:** используй superpowers:subagent-driven-development (рекомендуется) или superpowers:executing-plans. Шаги отмечай `- [ ]`.

**Цель:** третий вид упражнений `form` (выбрать форму слова в предложении) и общая рамка части 2: допуск умений, слабые слова из карточек, `lexemeId` в событиях, кнопка «Добавить в карточки».

**Архитектура:** поверх частей 1 и 2а. Чистая сборка и допуск в `internal/exercises`, новый кандидат `formCandidate` в `GetNextExercise` (тема `skill:form`), проверка тем же путём `AnswerExercise`, компонент `ExerciseCard` получает подсказку и кнопку. Тот же Glicko-2, те же таблицы и события.

**Стек:** Go, pgx + sqlc, goose, OpenAPI → oapi-codegen, React + TanStack Query. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-24-exercises-form-design.md` (код частей 1–2а: `internal/exercises`, `internal/httpapi/exercises*.go`, `web/src/features/exercises/`).

## Global Constraints

- Новых зависимостей нет. SQL только в `queries/`; контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` и `sqlc generate`, в `web/` — `npm run gen:api`; генерированное руками не правим.
- Интерфейс на русском, учебный контент на английском; сверка с `DESIGN.md` и его обновление. **Исполнитель фронтенда обязан вызвать навыки `impeccable`, `web-design-guidelines`, `emil-design-eng` через Skill-инструмент** и описать это в отчёте.
- Все новые тесты сначала RED (вывод падения в отчёте), потом GREEN.
- Миграция `00009` обратимая; **к настоящей базе применять только после явного «да» пользователя** и `pg_dump` (дамп без таблиц Tatoeba, в `.cache/backups/`). До этого только временная схема; к настоящей базе допустимо чтение (SELECT, EXPLAIN).
- Вариантов 3–4 (ответ + не меньше двух других форм слова); правильная форма клиенту до ответа не отдаётся.
- Глагол берётся, только если в предложении есть указатель времени из списка: `yesterday ago last tomorrow next now today tonight every always usually often never already yet just`.
- Слова с совпадающими формами пропускаются: `read lead saw put set cut hit let shut hurt cost spread quit bet beat`; также слово, у которого меньше трёх разных однословных форм.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`; в `web/` — `npx vitest run && npx tsc -b && npm run lint && npm run build`. Коммитить только свои файлы (не `git add -A`).
- Не пушить; коммиты по-русски, в конце `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

## Review Focus

- Форма встречается в предложении дважды («went … went») — отбрасывается (Task 1).
- В предложении две разные формы одного слова («go … went») — отбрасывается: ответ неоднозначен (Task 1).
- Глагол без указателя времени отбрасывается, существительное без указателя берётся (Task 1).
- Слово в начале предложения (заглавная буква): ответ и вариант с заглавной, проверка без учёта регистра (Task 1).
- У слова из слабых карточек нет подходящего предложения → берём следующее слово, затем кандидаты по уровню, затем следующая тема; не 500 (Task 3).
- Слово уже в карточках: у результата `word.inCards = true`, кнопки нет; повтор ответа по ключу возвращает те же `word` и `correct` (Task 3, 4).
- Нет карточек и нет настроек уровня: упражнение не падает, ответ 404 `no_exercise` только если нет вообще ничего (Task 3).

## Структура файлов

- `internal/exercises/form.go`, `form_test.go` — сборка; `unlock.go`, `unlock_test.go` — допуск умений.
- `migrations/00009_exercises_form.sql`, `queries/exercises.sql` (+`ListWeakCardWords`), `queries/cards.sql` при необходимости.
- `api/openapi.yaml` (+сгенерированное, `web/src/api/schema.gen.ts`).
- `internal/httpapi/exercises_pick.go` (+`formCandidate`), `exercises.go`, `exercises_answer.go`, `auth.go` (интерфейс хранилища), тесты и `exercises_fake_test.go`, `auth_test.go`.
- `web/src/features/exercises/ExercisesPage.tsx`, `AddToCards.tsx` (+тесты), `topics.ts`, `web/e2e/exercises-form.spec.ts`, `web/e2e/mock-api.ts`, `DESIGN.md`, `CLAUDE.md`.

---

### Task 1: Сборка `form` и допуск умений в `internal/exercises`

**Files:** Create `internal/exercises/form.go`, `form_test.go`, `unlock.go`, `unlock_test.go`.

**Interfaces:**
- Produces: `func BuildForm(sentence, pos string, forms []string, seed int64) (gap Gap, answer string, ok bool)` — `forms` — однословные формы слова в нижнем регистре (`words.SingleWords(words.Forms(lemma, pos))`); `answer` — найденная форма в написании предложения. `type Gate struct{ Requires string; MinRating float64; MinAttempts int }`; `func Unlocked(topic string, ratings []TopicRating) bool` (таблица допусков `gates` пока пуста: до первого закрытого умения всё доступно); внутренняя `unlocked(topic string, ratings []TopicRating, gates map[string]Gate) bool` для теста.

- [ ] **Step 1: тесты (RED).** `TestBuildFormBasic` («Yesterday she went to the cinema with her sister.», forms go/goes/went/gone/going, pos verb → `answer == "went"`, `Choices` содержат `went` и ≥ 2 других формы, ≤ 4, без повторов; `Answers[0]` совпадает с вариантом дословно); `TestBuildFormRejectsDoubleForm` («He went there and then went home yesterday.» → false); `TestBuildFormRejectsTwoFormsOfWord` («She goes home and went there yesterday.» → false); `TestBuildFormVerbNeedsTimeMarker` («She went to the cinema with her sister.» → false; то же с `yesterday` → true; существительное: «I saw two dogs in the garden.» forms dog/dogs, pos noun → true); `TestBuildFormCapitalized` («Went home early yesterday.» → `answer == "Went"`, `Answers[0] == "Went"`, `Check(gap.Answers, "went")` true); `TestBuildFormRejectsAmbiguousLemma` (forms слова `read`: read/reads/reading → false при любом предложении; слово с менее чем тремя разными формами → false); `TestBuildFormDeterministic` (один seed — те же варианты); `TestUnlockedDefaultOpen` (`skill:form`, `skill:match`, `grammar:articles` без рейтингов → true); `TestUnlockedGate` (через `unlocked` с таблицей `{"skill:x": {Requires: "skill:order", MinRating: 1550, MinAttempts: 10}}`: нет рейтинга → false; 1560 при 9 попытках → false; 1560 при 10 → true).
- [ ] **Step 2:** `go test ./internal/exercises -run 'Form|Unlocked'` падает. **Step 3:** реализовать по образцу `BuildGapForWord` (внутренняя `build(sentence, target, distractors, minDistr, seed)` с `minDistr = 2`; пропуск и варианты из неё, остальное проверки из Global Constraints). **Step 4:** тесты, `go vet`, `golangci-lint` проходят. **Step 5:** коммит `Упражнения: сборка «выбора формы» и допуск умений`.

### Task 2: Миграция `00009`, запрос слабых карточек, контракт

**Files:** Create `migrations/00009_exercises_form.sql`; Modify `queries/exercises.sql`, `api/openapi.yaml`, `internal/testdb/exercises_form_test.go` (создать), `internal/httpapi/auth.go`, `exercises_fake_test.go`; регенерация `internal/db`, `internal/api`, `web/src/api/schema.gen.ts`.

**Interfaces:**
- Produces: SQL `ListWeakCardWords(user_id, max_rows)` → `{LexemeID, Lemma, Pos}` — слова карточек пользователя со `state = 3` или `lapses > 0`, сначала с наибольшим `lapses`, затем с ближайшим `due`; только однословные леммы `^[a-z]+$` и части речи `verb`, `noun`, `adjective`; метод в интерфейсе `AuthStore` и в фейке. OpenAPI: `Exercise.kind` ∈ {gap, order, form}; `Exercise.hint` (string, необязательное: начальная форма, только для `form`); `AnswerResult.word` (необязательный объект `{lemma, pos, inCards}`, только для `form`).

- [ ] **Step 1: тесты (RED), временная схема:** миграция up/down/up по образцу `00008` (после Down `kind='form'` отклоняется; Down со строкой `form` падает с понятной ошибкой); `ListWeakCardWords`: слово со `state=3` и слово с `lapses=2` возвращаются в порядке лапсов, новая карточка без лапсов нет, чужие карточки нет, слово не из `verb/noun/adjective` нет, лимит работает.
- [ ] **Step 2:** падают. **Step 3:** миграция (`DROP CONSTRAINT exercises_kind_check; ADD CONSTRAINT … CHECK (kind IN ('gap','order','form'))`, Down возвращает `('gap','order')`), запрос, правки OpenAPI, `sqlc generate`, `go generate ./internal/api`, `npm run gen:api`. **Step 4:** `go build ./... && go test -race ./internal/testdb/... ./internal/httpapi/...`; `npx tsc -b` в `web/`. **Step 5:** `EXPLAIN (ANALYZE)` запроса на настоящей базе (только чтение), цель ≤ 20 мс, в отчёт. **Step 6:** коммит `Упражнения: миграция 00009, слабые слова карточек и контракт «выбора формы»`. Миграцию к настоящей базе НЕ применять.

### Task 3: Подбор `form`, проверка и событие

**Files:** Modify `internal/httpapi/exercises_pick.go` (`formCandidate`), `exercises.go` (`availableTopics`, `exercisePayload.Hint`, `exerciseAnswer` + `lexemeId/lemma/pos`, `saveExercise`), `exercises_answer.go`; тесты рядом.

**Interfaces:**
- Consumes: Task 1 `BuildForm`, `Unlocked`; Task 2 `ListWeakCardWords`, `ListCardLexemeIDs`, `RandomLexemesByLevel`, `ListExamples`.
- Produces: `func (s *authServer) formCandidate(ctx, rng *idRange, userID int64, level string, used map[int64]bool) (gapCandidate, bool, error)`; `gapCandidate` получает `form *formInfo{lexemeID int64; lemma, pos string}`; тема `skill:form` в `availableTopics` (через `Unlocked`); событие `exercise.answered` для `form` содержит `lexemeId`.

- [ ] **Step 1: тесты (RED), `fakeStore`:** `TestNextFormTopic` (рейтингов нет, `skill:form` выбран → ответ `kind: form`, `hint` = начальная форма, `choices` 3–4 и содержат форму, в JSON нет `answers`; запись `exercises` с `topics ⊇ {skill:form, level:…}`); `TestNextFormPrefersWeakCards` (слово из `ListWeakCardWords` берётся раньше случайного слова уровня); `TestNextFormNoSentenceFallsThrough` (у слабого слова нет предложения → следующее слово, затем `level:`-кандидат, не 500); `TestNextFormNoCardsNoSettings` (без карточек и настроек → слова уровня A1, при пустоте 404 `no_exercise`); `TestAnswerFormCorrectWrong` (рейтинги `skill:form` и `level:` обновлены, событие содержит `kind: form` и `lexemeId`); `TestAnswerFormWord` (`AnswerResult.word` = `{lemma, pos, inCards}`, `inCards=true` для слова из карточек); `TestAnswerFormRepeatByKey` (повтор по ключу возвращает те же `word` и `correct`, 409 при другом ключе). Интеграционный на настоящей Postgres во временной схеме: выдача `form` → ответ → повтор → 409.
- [ ] **Step 2:** падают. **Step 3:** реализовать; `lexemeId`, `lemma`, `pos` лежат в `exercises.answer` (JSON), миграции для этого нет; `AnswerResult.word.inCards` считается `ListCardLexemeIDs`. **Step 4:** `go test -race ./...`, `golangci-lint run`. **Step 5:** замер на настоящей базе (только чтение): сколько слов и предложений проходят отбор по уровням A1–C1 и время подбора (цель ≤ 100 мс); в отчёт. Если отбор по указателю времени оставляет меньше 1000 предложений на уровень, остановиться и сообщить пользователю. **Step 6:** коммит `Упражнения: подбор и проверка «выбора формы»`.

### Task 4: Фронтенд: подсказка и «Добавить в карточки»

**Files:** Create `web/src/features/exercises/AddToCards.tsx`, `AddToCards.test.tsx`; Modify `ExercisesPage.tsx`, `topics.ts` (`skill:form` → «Выбор формы»), `topics.test.ts`, `ExercisesPage.test.tsx`, `DESIGN.md`.

**Interfaces:**
- Consumes: Task 2 типы (`Exercise.hint`, `AnswerResult.word`), существующая `addCard(lemma, pos)` из `web/src/api/client.ts`.
- Produces: `AddToCards({ word: { lemma: string; pos: string; inCards: boolean } })`.

- [ ] **Step 1: тесты Vitest (RED):** для `kind: form` под предложением видна подсказка «(go)» и перевод; клавиши 1–4 работают как у `gap`; после ответа при `word.inCards=false` есть кнопка «Добавить в карточки», клик вызывает `addCard` один раз и меняет подпись на «В карточках» с галочкой; при `inCards=true` кнопки нет, есть «В карточках»; отказ сервера 409 `no_examples` показывает сообщение coral-text под кнопкой; для `gap`/`order` кнопки нет.
- [ ] **Step 2:** падают. **Step 3:** реализовать по образцу кнопки «В карточки» на странице «Слова», **вызвав навыки `impeccable`, `web-design-guidelines`, `emil-design-eng`**; фокус после ответа остаётся на карточке; `DESIGN.md`: раздел «Выбор формы». **Step 4:** `npx vitest run && npx tsc -b && npm run lint && npm run build`. **Step 5:** коммит `Упражнения: интерфейс «выбора формы»`.

### Task 5: Сквозные тесты, запуск, документы

**Files:** Create `web/e2e/exercises-form.spec.ts`; Modify `web/e2e/mock-api.ts`, `CLAUDE.md`.

- [ ] **Step 1:** подмена `form` в `mock-api.ts` (очередь фикстур с `kind: form`, `hint`, `word`); Playwright: ответ клавишей → «Верно» → «Добавить в карточки» → Пробел → следующее; неверный ответ показывает верную форму; слово уже в карточках без кнопки; снимки в светлой и тёмной теме на 1440/1024/390 px без горизонтальной прокрутки. **Step 2:** `npm run e2e` целиком проходит.
- [ ] **Step 3: запуск в настоящем приложении.** Спросить у пользователя «да» на применение `00009` (перед этим `pg_dump`); применить; перезапустить сервер; вход выполняет пользователь (создавать сессию в базе в обход входа нельзя; если он даёт логин и пароль — войти через Chromium проекта, не записывая пароль), пройти «выбор формы», сверить в базе строки `exercises` вида `form`, `topic_ratings` для `skill:form`, `events` с `lexemeId`. Не крутить страницу «вслепую»: неотвеченные упражнения копятся в `exercises`. Без входа не заявлять проверку в живом приложении.
- [ ] **Step 4:** обновить `CLAUDE.md` (только проверенное и ограничения), полный набор проверок, запустить ревью ветки (агент `reviewer`) и исправить находки тестом, который сначала падает. **Step 5:** коммиты.
