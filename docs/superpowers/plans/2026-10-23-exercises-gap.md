# Упражнение «пропуск» и рейтинг по темам: план реализации

> **Для исполнителя:** используй superpowers:executing-plans (или subagent-driven-development). Шаги отмечай `- [ ]`.

**Цель:** первое упражнение («пропуск» в предложении Tatoeba), рейтинг Glicko-2 по темам, выбор слабой темы, события попыток.

**Архитектура:** упражнение создаётся при запросе и сохраняется в `exercises` вместе с ответом; ответ проверяет сервер и в одной транзакции обновляет упражнение, рейтинги тем и пишет событие (как карточки). Логика — чистые функции в `internal/rating`, `internal/grammar`, `internal/exercises`; ручки и SQL тонкие.

**Стек:** Go, pgx + sqlc, goose, OpenAPI → oapi-codegen, React + TanStack Query. Новых зависимостей нет.

**Спек:** `docs/superpowers/specs/2026-10-23-exercises-gap-design.md`

## Глобальные ограничения

- Новых зависимостей нет (Glicko-2 пишем сами).
- SQL только в `queries/`, без склейки строк; контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` и `npm run gen:api` в `web/`; генерированное руками не правим.
- Интерфейс на русском, учебный контент на английском; сверка с `DESIGN.md`, его обновляем.
- Ошибки API: `{"error":{"code","message"}}`; номер упражнения передаётся в теле, не в пути.
- Миграция `00007` обратимая, только новые таблицы. **Применять к настоящей базе только после явного «да» пользователя**; до этого тесты идут во временной схеме.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`; в `web/` — `npx vitest run && npx tsc -b && npm run lint`.
- Отклонение от спека: у `topic_ratings` есть `user_id` (как у `cards` и `events`), первичный ключ `(user_id, topic)`.
- Отклонение от спека: уровень предложения считается по словам, совпавшим с леммой в `lexemes` точно (лемматизатора нет). Слово без точного совпадения не мешает, если его место в частотах ≤ 20 000; иначе предложение не берётся.

## Review Focus

- Предложение без единой подходящей темы или слова для пропуска: упражнение не создаётся, ручка отдаёт `404 no_exercise`, а не 500.
- Пустой или очень длинный `given` (больше 200 символов) и ответ на чужое/несуществующее упражнение: `400`/`404`, рейтинг не меняется.
- Двойная отправка ответа (двойной клик, повтор по ключу): рейтинг обновляется один раз.
- Предложения с апострофом, дефисом, регистром (`I'm`, `Mr.`): пропуск и проверка не ломаются, `Check` без учёта регистра.
- Пустая база рейтингов (первый запуск): `next` и `topics` работают без попыток.

## Структура файлов

- `internal/rating/rating.go` — Glicko-2.
- `internal/grammar/grammar.go` — грамматические темы по тексту.
- `internal/exercises/gap.go`, `pick.go` — сборка пропуска, проверка, выбор темы.
- `migrations/00007_exercises.sql`, `queries/exercises.sql` — таблицы и запросы.
- `internal/httpapi/exercises.go` (+ `exercises_test.go`, `exercises_fake_test.go`) — ручки.
- `api/openapi.yaml` — контракт.
- `web/src/features/exercises/` — страница; `web/e2e/exercises.spec.ts`.

---

### Task 1: Glicko-2 `internal/rating`

**Files:** Create `internal/rating/rating.go`, `internal/rating/rating_test.go`

**Interfaces:**
- Produces: `type Rating struct { Mu, Phi, Sigma float64 }` (Mu — рейтинг на шкале 1500, Phi — отклонение, Sigma — волатильность); `func New() Rating` (1500, 350, 0.06); `func Update(r Rating, opponent Rating, score float64) Rating` (один матч за период; `score` 1 — верно, 0 — неверно); `const Tau = 0.5`.

- [ ] **Step 1: тесты.** `TestUpdateWinRaisesRating` (1500/200/0.06 против 1400/30 с победой: Mu > 1500, Phi < 200); `TestUpdateLossLowersRating` (против 1700/300 с поражением: Mu < 1500); `TestNewDefaults`; `TestUpdateKeepsBounds` (Phi не выше 350, Sigma > 0). Сверка с числами примера из статьи Глика (три соперника за один период) недоступна для `Update` с одним матчем, поэтому формулы шагов 2–8 проверяются отдельной внутренней функцией `updatePeriod(r, []match)`, её тест берёт пример из статьи: ожидаемо Mu ≈ 1464.06, Phi ≈ 151.52, Sigma ≈ 0.05999.
- [ ] **Step 2:** `go test ./internal/rating` падает (пакета нет).
- [ ] **Step 3:** реализовать по формулам Glicko-2 из статьи Глика (шаги 2–8, итерация Иллинойс для σ' с допуском 1e-6, `Tau = 0.5`). Чистые функции, без времени.
- [ ] **Step 4:** тесты проходят.
- [ ] **Step 5:** коммит `Рейтинг: Glicko-2`.

### Task 2: Темы грамматики `internal/grammar`

**Files:** Create `internal/grammar/grammar.go`, `internal/grammar/grammar_test.go`

**Interfaces:**
- Consumes: `words.IrregularPast(word string) bool` — добавить в `internal/words` (экспорт существующей таблицы неправильных глаголов; если её нет в нужном виде, использовать то, что есть в `forms.go`, и дописать функцию рядом).
- Produces: `func Topics(tokens []string) []string` — ключи `grammar:articles|prepositions|future|modals|past_simple` в порядке перечисления, без повторов; `func Words(topic string) []string` — слова-кандидаты пропуска темы в нижнем регистре (артикли `a an the`; предлоги — закрытый список ~30; `will`, `going`; модальные).

- [ ] **Step 1: таблица-тест `TestTopics`:** `"I have a dog"` → `[grammar:articles]`; `"She is in the garden"` → articles, prepositions; `"I will call you"` → `[grammar:future, grammar:modals]` (will считается и модальным? нет: `will` только `future`; модальные — can/could/must/should/may/might/would); `"He went home"` → `[grammar:past_simple]`; `"They walked"` не даёт темы, если основа `walk` неизвестна (передаётся только токен, так что `-ed` считается темой лишь для слов длиннее 4 букв; тест фиксирует правило); пустой вход → `nil`; регистр не важен.
- [ ] **Step 2:** запуск, падение.
- [ ] **Step 3:** реализовать как множества слов; `past_simple` — неправильная форма из таблицы или токен длиннее 4 букв на `ed`.
- [ ] **Step 4:** проходят. **Step 5:** коммит `Грамматика: темы по тексту предложения`.

### Task 3: Сборка и проверка упражнения `internal/exercises`

**Files:** Create `internal/exercises/gap.go`, `internal/exercises/pick.go` и тесты.

**Interfaces:**
- Consumes: `grammar.Topics`, `grammar.Words`.
- Produces:
  - `type Gap struct { Prompt string; Choices []string; Answers []string }` (`Prompt` — предложение с `_____` на месте пропуска).
  - `func BuildGap(sentence, topic string, tokens []string, seed int64) (Gap, bool)` — выбирает позицию слова по теме (`grammar:*` — слово из `grammar.Words`; `level:*`/`pos:*` — слово, переданное как `target` ниже), собирает 4 варианта: правильный и 3 отвлекающих того же типа (для артиклей `a, an, the`, остальное из `grammar.Words` темы), порядок вариантов перемешан по `seed`; `false`, если слова нет.
  - `func BuildGapForWord(sentence, target string, distractors []string, seed int64) (Gap, bool)` — для тем `level:`/`pos:` (отвлекающие передаёт вызывающий: формы и однотипные слова).
  - `func Check(answers []string, given string) bool` — без регистра и крайних пробелов, ответ длиннее 200 символов — `false`.
  - `type TopicRating struct { Topic string; Mu, Phi float64; Attempts int }`; `func PickTopic(ratings []TopicRating, available []string) (string, bool)` — тема без попыток первой (по порядку `available`), иначе наименьший `Mu`, при равенстве наибольший `Phi`; пустой `available` → `false`.

- [ ] **Step 1: тесты:** `TestBuildGapArticles` (предложение `I have a dog.` → `Prompt = "I have _____ dog."`, `Answers = ["a"]`, в `Choices` ровно 4 уникальных, содержит `a`, `an`, `the`); `TestBuildGapKeepsPunctuationAndApostrophe` (`I'm in the garden.`); `TestBuildGapNoWord` (`false`); `TestCheck` (`" The "` против `["the"]` верно; пусто — неверно; 201 символ — неверно); `TestPickTopic` (новая тема раньше слабой; слабая раньше сильной; равные `Mu` — с большим `Phi`; пусто → `false`); `TestBuildGapDeterministic` (один `seed` → тот же порядок).
- [ ] **Step 2:** падение. **Step 3:** реализовать; пропуск заменяет ровно одно вхождение слова целиком (по границам слов), регистр первой буквы слова учитывается при показе вариантов (`The` в начале предложения → варианты с заглавной).
- [ ] **Step 4:** проходят. **Step 5:** коммит `Упражнения: сборка пропуска, проверка, выбор темы`.

### Task 4: Миграция `00007`, запросы и проверка скорости

**Files:** Create `migrations/00007_exercises.sql`, `queries/exercises.sql`; сгенерированное `internal/db/*` через `sqlc generate`.

**Interfaces:**
- Produces (sqlc): `InsertExercise(ctx, InsertExerciseParams{Kind, SentenceID, Topics []string, Payload, Answer []byte}) (int64, error)`; `GetExerciseForUpdate(ctx, GetExerciseForUpdateParams{ID}) (Exercise, error)` (блокирует строку); `MarkExerciseAnswered(ctx, id)`; `ListTopicRatings(ctx, userID) ([]TopicRating, error)`; `GetTopicRatingForUpdate(ctx, userID, topic)`; `UpsertTopicRating(ctx, UpsertTopicRatingParams{UserID, Topic, Rating, Deviation, Volatility, AttemptsDelta})`; `SampleSentences(ctx, SampleSentencesParams{FromID, Limit}) ([]SentenceWithTranslation, error)` — предложения 4–12 слов с переводом, начиная со случайного `id` (кандидаты для `grammar:*`); `SentenceLevels(ctx, words []string) ([]LemmaLevel, error)` — по списку слов нижнего регистра уровень (`min` из `lexeme_cefr` по лемме, как в поиске) и место в частотах (`word_frequencies`); `UsedSentences(ctx, topic string) ([]int64, error)` — предложения, уже использованные в теме.
- Таблицы по спеку, плюс `user_id` в `exercises` и `topic_ratings`; `exercises.answered_at`; индекс `exercises (user_id, answered_at)`.

- [ ] **Step 1:** написать миграцию (`-- +goose Up/Down`, `Down` = `DROP TABLE`); тест миграции up/down/up по образцу существующих (`internal/testdb`).
- [ ] **Step 2:** запросы `queries/exercises.sql`, `sqlc generate`.
- [ ] **Step 3: проверка скорости на настоящих данных.** Во временной схеме (не в основной базе) или на копии выполнить `EXPLAIN ANALYZE` для `SampleSentences` и `SentenceLevels`; цель — каждый до 100 мс. Если `SampleSentences` не укладывается, заменить выборку «от случайного `id`» на выборку по `ORDER BY id` с `OFFSET`-диапазоном по индексу и повторить. Результат записать в коммит.
- [ ] **Step 4:** `go test -race ./internal/db/... ./internal/testdb/...` проходят.
- [ ] **Step 5:** коммит `Упражнения: миграция 00007 и запросы`. **Применение к настоящей базе — отдельно, после «да».**

### Task 5: Контракт API и хранилище

**Files:** Modify `api/openapi.yaml`, `internal/httpapi/store.go` (интерфейс `AuthStore` — методы из Task 4), `internal/httpapi/cards_fake_test.go` рядом создать `exercises_fake_test.go` с реализацией методов в памяти (по образцу карточек, `InTx` со снимком).

**Interfaces:**
- Produces в OpenAPI: `GET /api/v1/exercises/next` → `Exercise {id, kind, prompt, translation, choices[], topic}`; `POST /api/v1/exercises/answer` тело `{exerciseId, given, key}` → `AnswerResult {correct, answer, translation, ratings[]}`, `Rating {topic, rating, deviation, attempts}`; `GET /api/v1/exercises/topics` → `{topics: [Rating]}`; ошибки `no_exercise` (404), `already_answered` (409), `invalid_answer` (400).

- [ ] **Step 1:** правки `openapi.yaml`, `go generate ./internal/api`, `npm run gen:api` в `web/`.
- [ ] **Step 2:** `go build ./...` (заглушки ручек, пока не реализованных, возвращают 501 с кодом `not_implemented`).
- [ ] **Step 3:** коммит `Упражнения: контракт API`.

### Task 6: Выдача упражнения и темы

**Files:** Create `internal/httpapi/exercises.go`, `internal/httpapi/exercises_test.go`

**Interfaces:**
- Consumes: Task 3 (`PickTopic`, `BuildGap`, `BuildGapForWord`), Task 4 (запросы), `words.Forms`.
- Produces: `func (s *authServer) GetNextExercise(w, r)`, `func (s *authServer) ListExerciseTopics(w, r)`.

- [ ] **Step 1: тесты на фейковом хранилище:** `TestNextExerciseGrammarTopic` (в рейтингах пусто, в выборке предложение `I have a dog.` → 200, `prompt` содержит `_____`, поля `answer` в ответе нет, в хранилище создана запись `exercises`); `TestNextExerciseNoCandidates` (404 `no_exercise`); `TestNextExerciseWeakestFirst` (тема с низким рейтингом выбирается раньше); `TestNextExerciseRequiresLogin` (401); `TestTopicsEmpty` (200 и пустой список).
- [ ] **Step 2:** падают. **Step 3:** реализовать: темы-кандидаты = все `grammar:*` из набора плюс `level:` пользователя из настроек (`settings`) и его соседний уровень; для `grammar:*` кандидаты из `SampleSentences` фильтруются `grammar.Topics` и уровнем предложения (правило из «Глобальных ограничений»); для `level:*` — случайная лексема уровня и предложение с её формами (логика из `examples.go`); если по теме ничего нет — перебрать следующую слабую тему, не больше 5 попыток, затем `no_exercise`. `pos:*` в v1 только как дополнительный ключ `topics` упражнения (тема не выбирается, но рейтинг ведётся), чтобы не усложнять выбор.
- [ ] **Step 4:** проходят. **Step 5:** коммит `Упражнения: выдача и список тем`.

### Task 7: Ответ на упражнение

**Files:** Modify `internal/httpapi/exercises.go`, `exercises_test.go`

**Interfaces:**
- Consumes: Task 1 (`rating.Update`), `exercises.Check`, `AppendEvent`.
- Produces: `func (s *authServer) AnswerExercise(w, r)`.

- [ ] **Step 1: тесты:** `TestAnswerCorrectUpdatesRatings` (верный ответ: рейтинги всех тем упражнения выросли, одно событие `exercise.answered` с `correct: true`, `given`, `ratingBefore/After`); `TestAnswerWrongDropsRatings`; `TestAnswerReplayByKey` (тот же ключ → тот же ответ, рейтинг не меняется, событие одно); `TestAnswerAlreadyAnswered` (другой ключ → 409); `TestAnswerUnknownExercise` (404); `TestAnswerInvalidBody` (пустой `given`, 201 символ, мусор после JSON → 400); `TestAnswerRollsBackOnEventFailure` (сбой записи события откатывает рейтинги и `answered_at`).
- [ ] **Step 2:** падают. **Step 3:** реализовать в `InTx`: `GetExerciseForUpdate` → проверить `answered_at` → `Check` → рейтинг по каждой теме (соперник — `Rating{Mu: сложность уровня предложения, Phi: 100, Sigma: 0.06}`; сложность берётся из ключа `level:*` упражнения, по умолчанию 1500) → `UpsertTopicRating` → `MarkExerciseAnswered` → `AppendEvent` с ключом идемпотентности. Время берётся из часов сервера (как в карточках, после блокировки).
- [ ] **Step 4:** проходят, включая интеграционный тест на настоящей базе во временной схеме (по образцу `integration_test.go`: выдача → ответ → повтор → 409). **Step 5:** коммит `Упражнения: проверка ответа, рейтинг и события`.

### Task 8: Фронтенд, раздел «Упражнения»

**Files:** Create `web/src/features/exercises/ExercisesPage.tsx`, `exercises.ts` (запросы TanStack Query), `ExercisesPage.test.tsx`, `WeakTopics.tsx`; Modify маршрут `/exercises` (заменить заглушку; найти по `Упражнения` в `AppShell.tsx` и месту маршрутов), `DESIGN.md`.

**Interfaces:**
- Consumes: сгенерированные типы `Exercise`, `AnswerResult`, `Rating`; ключ идемпотентности создаётся на упражнение, как в карточках.

- [ ] **Step 1: тесты Vitest:** `показывает предложение с пропуском и четыре варианта`; `клавиши 1–4 выбирают вариант и отправляют ответ`; `после ответа виден верный ответ, перевод и «Дальше» (Пробел)`; `404 no_exercise показывает пустое состояние`; `401 ведёт на вход`; `двойное нажатие отправляет один ответ`; `слабые темы: пять с наименьшим рейтингом`.
- [ ] **Step 2:** падают. **Step 3:** реализовать по образцу `CardsPage.tsx` (фокус на контейнере, чтобы работали клавиши, обработка `e.repeat`), тексты на русском, учебный контент на английском; стиль по `DESIGN.md`, его обновить.
- [ ] **Step 4:** `npx vitest run && npx tsc -b && npm run lint`. **Step 5:** коммит `Упражнения: страница и слабые темы`.

### Task 9: Сквозные тесты, запуск и документы

**Files:** Create `web/e2e/exercises.spec.ts`; Modify `web/e2e/mock-api.ts`, `CLAUDE.md`.

- [ ] **Step 1:** Playwright с подменой API: открыть «Упражнения», ответить клавишей верно и неверно, увидеть верный ответ и «Дальше», пустое состояние.
- [ ] **Step 2:** `npm run e2e` проходит. 
- [ ] **Step 3: запуск в настоящем приложении.** Спросить у пользователя «да» на применение миграции `00007` к локальной базе, применить (`goose ... up`), `go run ./cmd/server`, `npm run dev`; пройти упражнения в браузере через Playwright MCP; сверить в базе строки `exercises`, `topic_ratings`, `events`. Записать, сколько предложений нашлось по каждой теме и время `next`.
- [ ] **Step 4:** обновить статус в `CLAUDE.md` (только то, что реально проверено; команды не менялись), прогнать полный набор проверок, запустить `reviewer` один раз, исправить находки тестом, который сначала падает.
- [ ] **Step 5:** коммиты.
