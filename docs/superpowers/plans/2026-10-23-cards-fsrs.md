# Словарь: карточки слов и FSRS — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Слово добавляется в карточки со страницы «Слова»; раздел «Карточки» показывает очередь на сегодня (предложение с пропуском, ответ, оценка 1–4 по FSRS); «Сегодня» показывает настоящее число карточек к повторению; добавления и повторения пишутся в журнал событий.

**Architecture:** Тонкая обёртка `internal/srs` над `go-fsrs/v4` (чистые функции). Таблица `cards` хранит состояние FSRS; оценка выполняется в транзакции (блокировка строки, ключ идемпотентности через `events`). Хранилище HTTP-слоя получает метод `InTx`. API `POST /cards`, `GET /cards/queue`, `POST /cards/review`; на фронтенде страница «Карточки», кнопка «В карточки» и плитка на «Сегодня».

**Tech Stack:** Go 1.27, pgx v5, sqlc, goose, `github.com/open-spaced-repetition/go-fsrs/v4` (новая зависимость, одобрена пользователем), React 19 + TypeScript, TanStack Query, Vitest, Playwright.

**Spec:** `docs/superpowers/specs/2026-10-23-cards-fsrs-design.md`

## Global Constraints

- Единственная новая зависимость — `go-fsrs/v4`; при добавлении проверить лицензию (MIT) и транзитивные зависимости (`go list -m all`, `go mod why`) и записать в журнал выполнения. Больше ничего в `go.mod`/`package.json` не добавлять.
- Контракт сначала в `api/openapi.yaml`, затем `go generate ./internal/api` и `npm run gen:api`; генерированное руками не правим. Параметры пути и query в OpenAPI не объявляются (генератор тогда требует новую зависимость): номер карточки идёт в теле запроса.
- SQL только в `queries/`, `sqlc generate`; миграция `00005_cards.sql` обратимая, через навык `db-migration`, на реальную базу — только с явного «да» пользователя (Task 9).
- Библиотеку FSRS импортирует только `internal/srs`. Параметры по умолчанию (`DefaultParam()`), `EnableFuzz = false`.
- Оценки: 1 Again, 2 Hard, 3 Good, 4 Easy; любое другое значение — ошибка. Состояния FSRS: 0 New, 1 Learning, 2 Review, 3 Relearning.
- Журнал: события `card.added` и `card.reviewed` (`schema_version` 1); ключ идемпотентности повторения `review:<cardId>:<ключ клиента>`, ключ клиента 1–64 символа.
- Предложение для карточки: до 5 примеров (`ListExamples`), берутся только те, где форма слова найдена (`words.FindMatch`); индекс `reps mod число_примеров`.
- Интерфейс на русском, на «ты»; `dangerouslySetInnerHTML` запрещён; подсветка и пропуск в символах (кодовых точках), не в единицах UTF-16. Дизайн по `DESIGN.md`, после изменений интерфейса обновить его.
- Комментарии и сообщения об ошибках по-русски. Коммиты по просьбе пользователя; перед коммитом `go vet ./... && go test -race ./... && golangci-lint run` и в `web/` `npx vitest run && npx tsc -b && npm run lint`.

## Review Focus

- Повтор оценки с тем же ключом (двойной клик, ретрай): оценка применяется один раз, событие одно, ответ тот же (Task 5, 7).
- Оценка чужой или несуществующей карточки — 404 и ничего не меняется; плохая оценка, пустой или слишком длинный ключ — 400 (Task 5).
- Ошибка внутри транзакции откатывает и обновление карточки: `InTx` с функцией, вернувшей ошибку после записи, оставляет базу прежней (Task 3).
- Повторное добавление слова не создаёт вторую карточку и вторую `card.added`; слово без примеров — 409 `no_examples`, не 500; неизвестное слово — 404 `word_not_found` (Task 4).
- Карточка, у которой примеры исчезли, и очередь без карточек возвращаются корректно (`sentence: null`, `card: null`), без паники (Task 5).
- Пропуск и подсветка не сдвигаются у предложений с эмодзи и кириллицей; клавиши 1–4 и Пробел работают только в нужном состоянии; двойное нажатие не отправляет оценку дважды (Task 7).

---

### Task 1: Зависимость и обёртка FSRS `internal/srs`

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/srs/srs.go`, `internal/srs/srs_test.go`

**Interfaces:**
- Produces: `type Rating int` с константами `Again = 1`, `Hard`, `Good`, `Easy`; `type State int` с `New = 0`, `Learning`, `Review`, `Relearning`; `type Card struct{ Due time.Time; Stability, Difficulty float64; ScheduledDays, Reps, Lapses int; State State; LastReview time.Time; RemainingSteps int }` (нулевой `LastReview` — карточку ещё не повторяли); `func New(now time.Time) Card`; `func Review(c Card, r Rating, now time.Time) (Card, error)`; `func Preview(c Card, now time.Time) (map[Rating]time.Duration, error)` (через сколько вернётся карточка при каждой из четырёх оценок). Ошибки пакета: `ErrBadRating`.
- Библиотека: `fsrs.NewFSRS(fsrs.DefaultParam())` с выключенным fuzz; `Repeat(card, now)` для `Preview`, `Next(card, now, grade)` для `Review`; поля карточки библиотеки v4: `Due, Stability, Difficulty, ScheduledDays, Reps, Lapses (uint64), State, LastReview, RemainingSteps`.

- [ ] **Step 1: Add dependency:** `go get github.com/open-spaced-repetition/go-fsrs/v4@v4.0.0`; `go list -m all | grep -i fsrs`; убедиться в лицензии MIT (файл LICENSE в модуле) и в отсутствии неожиданных транзитивных зависимостей (`go mod graph | grep fsrs`); записать результат в журнал выполнения. Если появилось что-то кроме стандартной библиотеки — остановиться и спросить пользователя.
- [ ] **Step 2: Write the failing tests** (`srs_test.go`, `now` фиксируется): `New(now)` — состояние New, `Due == now`, `Reps == 0`; `Preview` на новой карточке: четыре значения, все > 0, `Again < Hard <= Good < Easy`; `Review(New, Good)` — `Reps == 1`, `Due` позже `now`, `LastReview == now`, состояние не New; последовательность `Good` пять раз с каждым разом в момент срока (`now = card.Due`) даёт неубывающие интервалы и растущую `Stability`, итоговое состояние Review; `Again` на карточке в состоянии Review увеличивает `Lapses` на 1 и переводит в Relearning, срок после — позже `now` и раньше срока при `Good`; `Review` с оценкой 0, 5 и -1 — `ErrBadRating`, карточка не меняется; `Review` не мутирует переданную карточку; `Preview` не мутирует карточку.
- [ ] **Step 3: Run** `go test ./internal/srs/...` — Expected: FAIL.
- [ ] **Step 4: Implement** по Interfaces (конвертация `uint64` ↔ `int` с проверкой границ).
- [ ] **Step 5: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто.
- [ ] **Step 6: Commit**: «Зависимость go-fsrs/v4 и обёртка internal/srs».

### Task 2: Миграция `00005` и запросы sqlc

**Files:**
- Create: `migrations/00005_cards.sql`, `queries/cards.sql`
- Modify: `internal/testdb/testdb_test.go`
- Regenerate: `internal/db/*.go` (`sqlc generate`)

**Interfaces:**
- DDL: `cards(id bigint identity PK, user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE, lexeme_id bigint NOT NULL REFERENCES lexemes ON DELETE RESTRICT, due timestamptz NOT NULL, stability double precision NOT NULL CHECK >= 0, difficulty double precision NOT NULL CHECK >= 0, scheduled_days integer NOT NULL CHECK >= 0, reps integer NOT NULL CHECK >= 0, lapses integer NOT NULL CHECK >= 0, state smallint NOT NULL CHECK BETWEEN 0 AND 3, last_review timestamptz, remaining_steps integer NOT NULL CHECK >= 0, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE (user_id, lexeme_id))`, индексы `(user_id, due)` и `(lexeme_id)`. Down: `DROP TABLE cards;` с комментарием, что откат удаляет прогресс повторений (события остаются в `events`).
- Produces (sqlc, имена полей сверить по генерации): `GetLexemeByPair(ctx, {Lemma, Pos}) (db.Lexeme, error)`; `InsertCard(ctx, {UserID, LexemeID, Due, Stability, Difficulty, ScheduledDays, Reps, Lapses, State, LastReview, RemainingSteps}) (int64, error)` — `ON CONFLICT (user_id, lexeme_id) DO NOTHING RETURNING id` (при конфликте `pgx.ErrNoRows`); `GetCardByLexeme(ctx, {UserID, LexemeID}) (db.Card, error)`; `GetCardForUpdate(ctx, {ID, UserID}) (GetCardForUpdateRow, error)` — карточка + `lemma`, `pos` слова, `FOR UPDATE OF c`; `UpdateCardState(ctx, {ID, Due, Stability, Difficulty, ScheduledDays, Reps, Lapses, State, LastReview, RemainingSteps}) error` (выставляет `updated_at = now()`); `NextDueCard(ctx, {UserID, Now}) (NextDueCardRow, error)` — карточка + `lemma`, `pos`, `level` (самый низкий из `lexeme_cefr`, строка) с `due <= now`, порядок `due, id`, `LIMIT 1`; `CountCards(ctx, {UserID, Now}) (CountCardsRow{Total, Due int64}, error)`; `ListCardLexemeIDs(ctx, {UserID, Ids []int64}) ([]int64, error)`.

- [ ] **Step 1: Write the failing tests** в `testdb_test.go`: `allTables` дополняется `cards` (up → down → up); `TestCardSchemaConstraints`: состояние 4 и -1, отрицательные стабильность, сложность, `reps`, `lapses`, `scheduled_days`, `remaining_steps` → 23514; повтор `(user_id, lexeme_id)` → 23505; несуществующие `user_id`/`lexeme_id` → 23503; удаление слова с карточкой запрещено; удаление пользователя каскадно убирает карточки; `last_review` допускает NULL. `TestCardQueries` (на `Queries`): `InsertCard` возвращает id, повтор даёт `pgx.ErrNoRows` и `GetCardByLexeme` находит исходную; `NextDueCard` возвращает карточку со сроком в прошлом и не возвращает со сроком в будущем, при двух карточках — с более ранним сроком, поле `level` равно минимальному из двух источников; `CountCards` считает всего и просроченных; `GetCardForUpdate` чужой пользователь → `pgx.ErrNoRows`; `UpdateCardState` меняет поля и `updated_at`; `ListCardLexemeIDs` отдаёт только слова пользователя.
- [ ] **Step 2: Run** `go test ./internal/testdb/...` — Expected: FAIL.
- [ ] **Step 3: Implement** миграцию и `queries/cards.sql` по Interfaces; `sqlc generate`.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто.
- [ ] **Step 5: Commit**: «Схема карточек: cards и запросы».

### Task 3: Контракт API и транзакционное хранилище

**Files:**
- Modify: `api/openapi.yaml`, `internal/httpapi/auth.go` (интерфейс `AuthStore`), `internal/httpapi/health.go` (`NewHandler`), `cmd/server/main.go`, тесты `internal/httpapi/*_test.go` (фейк и вызовы `NewHandler`)
- Create: `internal/httpapi/store.go`, `internal/httpapi/store_test.go`
- Regenerate: `internal/api/api.gen.go`

**Interfaces:**
- Produces в OpenAPI: `POST /api/v1/cards` (`addCard`; тело `{lemma, pos}`; 200/201 `{cardId, created}`; 400, 401, 403, 404 `word_not_found`, 409 `no_examples`), `GET /api/v1/cards/queue` (`getCardQueue`; 200 `CardQueue`; 401), `POST /api/v1/cards/review` (`reviewCard`; тело `{cardId, rating, key}`; 200 `CardQueue`; 400, 401, 403, 404); схемы `CardQueue{dueCount, totalCount, card: QueueCard|nullable}`, `QueueCard{cardId, lemma, pos, level, state, sentence: CardSentence|nullable, intervals: {again, hard, good, easy}}` (секунды), `CardSentence{en: ExampleSentence, ru: ExampleSentence, gap: TextRange}`; в `Word` обязательное `inCards: boolean`; в enum кодов ошибок `no_examples`, `word_not_found`. Номер карточки только в теле, пути и query не объявлять.
- Produces (Go): `AuthStore` расширяется методами `InTx(ctx context.Context, fn func(AuthStore) error) error` и всеми запросами `Queries` из Task 2 (`GetLexemeByPair`, `InsertCard`, `GetCardByLexeme`, `GetCardForUpdate`, `UpdateCardState`, `NextDueCard`, `CountCards`, `ListCardLexemeIDs`) плюс `AppendEvent`; `type PgStore struct{ *db.Queries; pool *pgxpool.Pool }`, `NewPgStore(pool *pgxpool.Pool) *PgStore`, метод `InTx` открывает транзакцию, вызывает `fn` с хранилищем на этой транзакции (`PgStore`/обёртка над `Queries.WithTx(tx)` без вложенных транзакций), откатывает при ошибке, фиксирует при успехе. `cmd/server` и интеграционные тесты передают `httpapi.NewPgStore(pool)` вместо `db.New(pool)`; `fakeStore.InTx` вызывает `fn(f)`.

- [ ] **Step 1: Write the failing tests** (`store_test.go`, база из `testdb.Pool`): `InTx` фиксирует запись при успехе; при ошибке из `fn` откатывает всё, что `fn` записал (пользователь/источник/слово/карточка, затем ошибка → таблицы пусты); при панике в `fn` транзакция откатывается и паника не глотается; вложенный `InTx` внутри `fn` работает на той же транзакции (не открывает вторую и не виснет).
- [ ] **Step 2: Run** `go test ./internal/httpapi/ -run InTx` — Expected: FAIL.
- [ ] **Step 3: Implement**: правка `openapi.yaml`, `go generate ./internal/api`; `store.go`; расширение `AuthStore`; обновить `cmd/server/main.go`, фейк и все вызовы `NewHandler` в тестах; заглушки методов `AddCard`, `GetCardQueue`, `ReviewCard` в `authServer` возвращают 501 (реализуются в Task 4–5), чтобы пакет собирался.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто, существующие тесты не сломаны.
- [ ] **Step 5: Commit**: «Контракт API карточек и транзакционное хранилище».

### Task 4: `POST /api/v1/cards` — добавить слово в карточки

**Files:**
- Create: `internal/httpapi/cards.go`, `internal/httpapi/cards_test.go`
- Modify: `internal/httpapi/integration_test.go`

**Interfaces:**
- Consumes: `AuthStore` (Task 3), `srs.New`, `words.Forms`/`words.TSQuery`/`words.FindMatch`, `ListExamples`.
- Produces: `(*authServer).AddCard`: сессия обязательна; тело `{lemma, pos}` (до 64 и 32 символов, корректный UTF-8, без нулевого байта, лемма непустая) — иначе 400 `invalid_request`; слово находится по точной паре; нет слова → 404 `word_not_found`; нет ни одного примера с найденной формой → 409 `no_examples`; иначе в транзакции `InsertCard` (`srs.New(now)`) и при создании `AppendEvent` типа `card.added` с нагрузкой `{cardId, lexemeId, lemma, pos}`; повтор для того же слова → 200 `created: false`, без второго события. Хелпер `exampleSentences(ctx, store, lemma, pos) ([]sentence, error)` (до 5 примеров, только с найденной формой) — переиспользуется в Task 5.

- [ ] **Step 1: Write the failing tests** (`cards_test.go`, фейк с `lexemes`, `cards`, `events`): 401 без сессии; 400 на пустую и 65-символьную лемму, нулевой байт, не-UTF-8, `pos` из 33 символов, битый JSON, лишнее поле; 404 для неизвестного слова (карточка не создана); 409 для слова без примеров (карточка не создана); успех — 201, `created: true`, событие `card.added` с нужной нагрузкой и ключом идемпотентности; повтор — 200, `created: false`, событий всё ещё одно; чужой `Origin` — 403. Интеграционный (`TestAddCardOnRealDatabase`): настоящая БД с источником, словом `run/verb` и подходящими предложениями — карточка создаётся в состоянии New со сроком не позже «сейчас», повтор не создаёт вторую строку и второе событие, слово без предложений даёт 409 и пустую таблицу `cards`.
- [ ] **Step 2: Run** `go test ./internal/httpapi/ -run 'AddCard'` — Expected: FAIL (501).
- [ ] **Step 3: Implement** `AddCard` и `exampleSentences`.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто.
- [ ] **Step 5: Commit**: «API: добавить слово в карточки».

### Task 5: Очередь и оценка карточек

**Files:**
- Modify: `internal/httpapi/cards.go`, `internal/httpapi/cards_test.go`, `internal/httpapi/integration_test.go`

**Interfaces:**
- Produces: `(*authServer).GetCardQueue` — `{dueCount, totalCount, card}`: `card == null`, когда нет просроченных; иначе `QueueCard`: слово, уровень, состояние, `intervals` из `srs.Preview` в секундах (`again/hard/good/easy`), `sentence` — предложение номер `reps mod n` из `exampleSentences` (`gap` = диапазон найденной формы в символах) или `null`, если примеров нет. `(*authServer).ReviewCard` — тело `{cardId, rating, key}`: оценка 1–4, ключ 1–64 символа (иначе 400); в транзакции `GetCardForUpdate` (чужая/несуществующая → 404, ничего не меняется), `srs.Review`, `AppendEvent` типа `card.reviewed` (нагрузка `{cardId, lexemeId, rating, stateBefore, stateAfter, dueAfter, scheduledDays, stability, difficulty}`, ключ `review:<cardId>:<key>`); если событие с таким ключом уже есть (`pgx.ErrNoRows`), карточка не обновляется; затем `UpdateCardState`; ответ — та же структура `CardQueue` (следующая карточка после оценки).

- [ ] **Step 1: Write the failing tests**: очередь — 401; пустая (нет карточек, нет просроченных) → `card: null`, счётчики верны; карточка в будущем не в очереди, но входит в `totalCount`; с карточкой: поля слова, `intervals` четыре положительных числа и `again < hard <= good < easy`, `sentence` с `gap` внутри текста; индекс предложения меняется с `reps` (при двух примерах `reps = 0` и `reps = 1` дают разные); примеров нет → `sentence: null` и 200; оценка — 401/403; 400 на оценки 0, 5, -1, пустой ключ, ключ из 65 символов, битый JSON; 404 на карточку другого пользователя и несуществующую (в базе ничего не изменилось); успех «Good» на новой карточке: срок позже, `reps = 1`, событие `card.reviewed` одно с верной нагрузкой; **повтор с тем же ключом**: состояние карточки и число событий не изменились, ответ успешный; тот же ключ на другой карточке — отдельное событие; следующий ответ содержит следующую карточку (если есть). Интеграционный (`TestReviewCardsOnRealDatabase`): настоящая БД, две карточки, цикл «Good» до смены состояния, срок и `scheduled_days` растут, события в `events` по порядку, повтор с ключом не меняет ничего, очередь пустеет после оценки единственной просроченной карточки, `Again` на карточке в Review даёт `lapses = 1`.
- [ ] **Step 2: Run** `go test ./internal/httpapi/ -run 'Queue|Review'` — Expected: FAIL.
- [ ] **Step 3: Implement** `GetCardQueue` и `ReviewCard` по Interfaces; общий построитель `queueResponse(ctx, store, userID, now)`.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто.
- [ ] **Step 5: Commit**: «API: очередь карточек и оценка повторения».

### Task 6: `inCards` в поиске слов

**Files:**
- Modify: `internal/httpapi/words.go`, `internal/httpapi/words_test.go`, `internal/httpapi/integration_test.go`

**Interfaces:**
- Consumes: `ListCardLexemeIDs` (Task 2); `SearchLexemes` возвращает `ID` слова (уже есть).
- Produces: в `api.Word` поле `InCards bool`: истина, если у пользователя есть карточка этого слова; один дополнительный запрос на ответ (только когда есть найденные слова).

- [ ] **Step 1: Write the failing tests**: фейк с карточкой на слово №1 — в ответе `inCards: true` у него и `false` у других; слов нет — запрос карточек не выполняется; интеграционный: слово после `POST /cards` в поиске получает `inCards: true`; карточка другого пользователя не влияет (проверка `UserID` в запросе).
- [ ] **Step 2: Run** `go test ./internal/httpapi/ -run Words` — Expected: FAIL.
- [ ] **Step 3: Implement** в `SearchWords`.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS, чисто.
- [ ] **Step 5: Commit**: «Поиск слов: признак inCards».

### Task 7: Фронтенд: страница «Карточки»

**Files:**
- Regenerate: `web/src/api/schema.gen.ts`
- Modify: `web/src/api/client.ts`, `web/src/api/client.test.ts`, `web/src/app/router.tsx`
- Create: `web/src/features/cards/cards.ts`, `web/src/features/cards/CardsPage.tsx`, `web/src/features/cards/Cloze.tsx`, `web/src/features/cards/format.ts`, тесты рядом

**Interfaces:**
- Produces: в `client.ts` `getCardQueue(): Promise<CardQueue>`, `reviewCard(cardId: number, rating: 1|2|3|4, key: string): Promise<CardQueue>`, `addCard(lemma: string, pos: string): Promise<{cardId: number; created: boolean}>`; `cards.ts` — `cardQueueKey`, `useCardQueue()`, `useReviewCard()` (при успехе кладёт ответ в кэш очереди), `useAddCard()` (при успехе делает недействительными очередь и поиск слов); `Cloze({ text, gap, revealed })` — предложение с пропуском `_____` или (раскрытое) с подсвеченным словом; диапазон в символах (`Array.from`); `formatInterval(seconds: number): string` — «10 мин», «3 дн», «2 мес.» с корректным склонением и округлением (меньше минуты — «меньше минуты»); `CardsPage` по спеку.

- [ ] **Step 1: Write the failing tests**: `format.test.ts` — 30 с, 60 с, 600 с, 3 600 с, 86 400 с, 3 дня, 45 дней, 400 дней дают ожидаемые строки («1 мин», «10 мин», «1 ч», «1 дн», «3 дн», «1 мес.», «1 г.»; границы и склонения); `Cloze.test.tsx` — пропуск по диапазону, раскрытие подсвечивает слово в `<mark>`, эмодзи до слова не сдвигает диапазон, кривой диапазон не ломает вывод, разметка в тексте остаётся текстом; `client.test.ts` — три вызова (методы, тела, URL); `CardsPage.test.tsx` — загрузка; пустая очередь («На сегодня всё»); карточка: пропуск и перевод видны, ответа нет; «Показать ответ» (кнопка, Пробел и Enter) раскрывает слово, показывает атрибуцию Tatoeba и четыре кнопки оценки с подписями интервалов; клавиши 1–4 работают только после ответа (до ответа не отправляют оценку); оценка отправляет `rating` и ключ, показывает следующую карточку и «Осталось: N»; двойной клик/двойное нажатие клавиши отправляют один запрос, а ключ на следующей карточке новый; ошибка оценки — сообщение и повторить с тем же ключом, карточка остаётся; ошибка загрузки — «Повторить»; карточка без предложения показывает слово, часть речи и уровень и тоже оценивается; в `document.title` «Карточки — Verst».
- [ ] **Step 2: Run** `cd web && npm run gen:api && npx vitest run src/features/cards src/api` — Expected: FAIL.
- [ ] **Step 3: Implement**; маршрут `cards` заменяет заглушку; ключ идемпотентности — `crypto.randomUUID()` на каждую показанную карточку; защита от двойной отправки — `useRef`-флаг плюс блокировка кнопок.
- [ ] **Step 4: Run** `cd web && npx vitest run && npx tsc -b && npm run lint` — Expected: PASS.
- [ ] **Step 5: Commit**: «Страница «Карточки»».

### Task 8: «В карточки» на странице «Слова» и плитка на «Сегодня»

**Files:**
- Modify: `web/src/features/words/WordsPage.tsx`, `web/src/features/words/WordsPage.test.tsx`, `web/src/features/today/TodayPage.tsx`, `web/src/features/today/TodayPage.test.tsx`, `web/src/features/today/demo.ts`

**Interfaces:**
- Consumes: `useAddCard`, `useCardQueue` (Task 7); поле `inCards` в `Word`.
- Produces: у строки слова кнопка «В карточки» (состояния: обычная; «Добавляем…»; «В карточках» неактивная при `inCards` или после успеха; ошибка 409 `no_examples` — сообщение «Для этого слова нет подходящих примеров»; иная ошибка — её текст и повтор); плитка «Карточки к повторению» на «Сегодня» берёт число из очереди (`dueCount`), при загрузке показывает прочерк, при ошибке — «—» без падения экрана; из `DEMO_TODAY` удаляется `cardsDue`.

- [ ] **Step 1: Write the failing tests**: слово без карточки — кнопка «В карточки» отправляет `POST /api/v1/cards` с леммой и частью речи, после успеха меняется на «В карточках» и блокируется; слово с `inCards` сразу «В карточках»; 409 `no_examples` показывает сообщение, кнопка остаётся доступной; двойной клик отправляет один запрос; «Сегодня» показывает `dueCount` из `/api/v1/cards/queue` (например 7) и ссылка ведёт на `/cards`; при ошибке очереди плитка не ломает экран.
- [ ] **Step 2: Run** `cd web && npx vitest run src/features/words src/features/today` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npx vitest run && npx tsc -b && npm run lint` — Expected: PASS.
- [ ] **Step 5: Commit**: «Слова: добавить в карточки; Сегодня: число карточек».

### Task 9: Сквозные тесты, проверка в браузере, реальная проверка и документы

**Files:**
- Modify: `web/e2e/mock-api.ts`, `web/e2e/screens.spec.ts` (если нужен мок очереди), `DESIGN.md`, `CLAUDE.md`
- Create: `web/e2e/cards.spec.ts`

- [ ] **Step 1: Write the failing e2e** (`cards.spec.ts`, API подменён состоянием в памяти: добавление → очередь → оценки → пустая очередь): из меню «Слова» → найти слово → «В карточки» → «Карточки» → пропуск виден → Пробел раскрывает ответ → клавиша 3 → следующая карточка/«На сегодня всё»; повторная оценка того же ключа не принята дважды; снимки экранов лицо/оборот/пусто в обеих темах на 1440, 1024 и 390 без горизонтальной прокрутки; плитка на «Сегодня» показывает число.
- [ ] **Step 2: Run** `cd web && npm run e2e` — Expected: сначала FAIL; после правок мока и страницы PASS, существующие тесты не сломаны; посмотреть снимки глазами.
- [ ] **Step 3: STOP и спросить пользователя:** можно ли применить миграцию `00005` к локальной базе (`set -a; . ./.env; set +a; goose -dir migrations postgres "$DATABASE_URL" up`; только создаёт таблицу `cards`) и выполнить проверку на настоящих данных (добавить 3–4 слова, цикл оценок, посмотреть события). Без явного «да» не выполнять.
- [ ] **Step 4: После согласия:** применить миграцию; перезапустить Go-сервер на новом коде; временной программой (удалить после) или через `psql` проверить: добавление слов с настоящими предложениями, очередь с `sentence` и `gap`, цикл «Good»/«Again» меняет состояние и срок, в `events` появились `card.added` и `card.reviewed` с нужной нагрузкой, повтор с тем же ключом ничего не меняет; `GET /api/v1/cards/queue` без входа даёт 401 напрямую и через Vite.
- [ ] **Step 5: Документы:** `DESIGN.md` — компоненты «Карточки» (лицо, оборот, кнопки оценки с интервалами, пустое состояние, кнопка «В карточки»); `CLAUDE.md` — статус (этап «Словарь» сделан: карточки, FSRS через go-fsrs/v4, журнал событий; ещё нет: частотного списка, упражнений, серий) и команды без изменений.
- [ ] **Step 6: Финальная проверка:** `go vet ./... && go test -race ./... && golangci-lint run`; в `web/`: `npx vitest run && npx tsc -b && npm run lint && npm run build && npm run e2e`; ревью (агент `reviewer`, один раз на подпроект).
- [ ] **Step 7: Commit** по просьбе пользователя: «Карточки: сквозные тесты и документы».
