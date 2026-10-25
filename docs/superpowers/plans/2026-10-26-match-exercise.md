# Упражнение «Сопоставление» Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Четвёртый вид упражнений `match`: соединить 5 английских слов с 5 русскими переводами из словаря `freedict-eng-rus`, с подробными анимациями.

**Architecture:** Чистые функции в `internal/exercises` (выбор перевода, сборка набора, проверка пар). Вид идёт по существующему конвейеру тем (`skill:match` в `availableTopics` рядом с `skill:form`), хранится в `exercises` с пустым `sentence_id`, оценивается дробным счётом Glicko-2. Фронтенд: чистая логика пар отдельно от компонента `MatchExercise`; анимации CSS и SVG, клавиатурный путь без анимации.

**Tech Stack:** Go, pgx + sqlc, goose, oapi-codegen; React, TypeScript, Vitest, Playwright, CSS-анимации. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-26-match-exercise-design.md`

## Global Constraints

- Новых зависимостей нет; SQL только в `queries/` через sqlc; контракт меняется сначала в `api/openapi.yaml`, сгенерированные файлы (`internal/api/api.gen.go`, `web/src/api/schema.gen.ts`, `internal/db/*`) коммитятся вместе с контрактом, перед коммитом `git status`.
- Миграции: `pg_dump` до применения (`.cache/backups/before-00012.dump`, без таблиц Tatoeba), откат падает, пока есть строки `match`.
- Интерфейс на русском, учебный контент на английском; тексты ошибок называют проблему и выход.
- Движение: только `transform` и `opacity`; клавиатурные действия без анимации; при `prefers-reduced-motion` остаются цвет, рамка, значок; один праздничный момент (итог 5 из 5).
- Секреты и пароль пользователя не писать в файлы и коммиты. Не пушить и не сливать без просьбы.
- Тесты: Go `go vet ./... && go test -race ./...`, `golangci-lint run`; фронтенд из `web/`: `npx tsc -b`, `npm run lint`, `npx vitest run`, `npm run build`, `npm run e2e`.

## Review Focus

- Ответ с чужим словом, повтором слова или перевода, не 5 пар, пробелы и регистр: 400 или оценка без паники.
- Одно слово набора из нескольких частей речи (`run` n и v): пара «лемма + часть речи» не смешивается.
- Параллельные ответы и повтор запроса по ключу: счёт не двоится, рейтинги не обновляются дважды (как у остальных видов).
- Код, читающий `exercises.sentence_id` (банк ошибок, повторы, `UsedSentences`), не падает на пустом значении.
- Недостаточно слов с переводом (новая база без словаря): вид не выдаётся, остальные виды работают.
- Узкий экран 390 px: колонки и нити не создают горизонтальную прокрутку; клавиатура и читалка получают те же пары, что и мышь.

---

### Task 1: Миграция 00012 и запрос переводов

**Files:**
- Create: `migrations/00012_exercises_match.sql`, `queries/translations_read.sql`
- Modify: `internal/db/*` (sqlc), код, который не собирается после смены типа `sentence_id` на указатель

**Interfaces:**
- Produces: `db.Exercise.SentenceID *int64`, `db.InsertExerciseParams.SentenceID *int64`; `ListTranslationsForWords(ctx, db.ListTranslationsForWordsParams{SourceName string, Lemmas []string}) ([]db.ListTranslationsForWordsRow, error)` со строками `{Lemma, Pos string; SenseNo int16; Ru string}` из последнего завершённого импорта `freedict-eng-rus` (как `ListWordFrequencies`), по ключам `lemma = ANY(lemmas)`.

- [ ] **Step 1:** Написать миграцию: `kind IN ('gap','order','form','match')`, `sentence_id DROP NOT NULL`; `Down` падает с понятным текстом при наличии строк `match`, затем возвращает `NOT NULL` и прежнюю проверку.
- [ ] **Step 2:** Написать запрос чтения и `sqlc generate`; `go build ./...` покажет места, где `SentenceID` теперь указатель; поправить их (вставка разыменовывает адрес, чтение с проверкой `nil`). Expected: `go vet ./...` без ошибок, `go test ./internal/httpapi` зелёный.
- [ ] **Step 3:** Интеграционный тест на Postgres во временной схеме (в `internal/httpapi/integration_test.go`): вставить источник и три перевода, прочитать запросом; вставить упражнение `match` с пустым `sentence_id`; убедиться, что откат миграции падает при такой строке (проверка текста ошибки).
- [ ] **Step 4:** `pg_dump` и `goose up` на локальной базе (команды как в `CLAUDE.md`), проверить `psql \d exercises`.
- [ ] **Step 5:** Commit: `Сопоставление: миграция 00012 и чтение переводов`.

### Task 2: Чистая логика в `internal/exercises`

**Files:**
- Create: `internal/exercises/match.go`, `internal/exercises/match_test.go`

**Interfaces:**
- Produces:
  - `const MatchPairs = 5`
  - `type MatchCandidate struct { Word string; Senses [][]string }` (значения слова по порядку, в каждом русские варианты)
  - `type MatchPair struct { Word, Ru string }`
  - `type Match struct { Words, Options []string; Answers []MatchPair }`
  - `func PickTranslation(senses [][]string) (string, bool)`
  - `func BuildMatch(cands []MatchCandidate, seed int64) (Match, bool)` (слова идут в порядке кандидатов, варианты перемешаны)
  - `type PairResult struct { Word, Given, Answer string; Correct bool }`
  - `func CheckMatch(answers []MatchPair, given string) ([]PairResult, int, error)` (число верных, ошибка с русским текстом при неверном формате)

- [ ] **Step 1:** Тесты `PickTranslation`: побеждает перевод из наибольшего числа значений; ничья решается более ранним значением; единственное значение с одним вариантом подходит; неоднозначный победитель (встречается в одном значении из нескольких) даёт `false`; ударения, регистр, пробелы нормализуются; пустой ввод. Запустить: FAIL (функции нет).
- [ ] **Step 2:** Реализовать `PickTranslation`; тесты зелёные.
- [ ] **Step 3:** Тесты `BuildMatch`: ровно 5 пар из первых подходящих; пропуск слов без перевода, с пробелом, длиннее 20 или короче 2 букв, с переводом длиннее 24 символов или из нескольких слов, совпадающим с английским; два слова с одним переводом не попадают вместе; меньше 5 подходящих — `false`; одинаковый `seed` даёт один порядок вариантов, другой `seed` может дать другой; варианты содержат ровно переводы набора. FAIL, затем реализовать, зелёные.
- [ ] **Step 4:** Тесты `CheckMatch`: все верные, часть верных (счёт и разбор по парам), регистр и пробелы вокруг игнорируются, повтор слова, чужое слово, повтор перевода, меньше или больше 5 пар, пустая строка, пара без `=`; каждая ошибка с понятным русским текстом. FAIL, реализовать, зелёные.
- [ ] **Step 5:** `go vet ./internal/exercises && go test -race ./internal/exercises`; commit: `Сопоставление: выбор перевода, сборка набора и проверка пар`.

### Task 3: Контракт и сервер

**Files:**
- Modify: `api/openapi.yaml`, `internal/api/api.gen.go`, `web/src/api/schema.gen.ts`, `internal/httpapi/exercises.go`, `internal/httpapi/exercises_pick.go`, `internal/httpapi/exercises_answer.go`, `internal/httpapi/auth.go` (`AuthStore` расширяется методом `ListTranslationsForWords`)
- Test: `internal/httpapi/exercises_match_test.go`, `internal/httpapi/exercises_match_fake_test.go`, `internal/httpapi/integration_test.go`

**Interfaces:**
- Consumes: Task 1 (`ListTranslationsForWords`, указатель `SentenceID`), Task 2 (`BuildMatch`, `CheckMatch`, `MatchPair`).
- Produces: в `Exercise`: `kind: match`, необязательные `words`, `options`; в `AnswerRequest.given` до 400 символов; в `AnswerResult` необязательное `pairs` (`{word, given, answer, correct}`); тема `skill:match`; событие `exercise.answered` с `correctPairs` и `pairs`.

- [ ] **Step 1:** Контракт: добавить `match` в `Exercise.kind`, поля `words`, `options`, схему `PairResult`, поле `pairs` в `AnswerResult`, `maxLength: 400` у `given`; `go generate ./internal/api`, `cd web && npm run gen:api`; `git status` показывает все три сгенерированных файла.
- [ ] **Step 2:** Тесты обработчика с поддельным хранилищем (RED): выдача (достаточно слов со словарём → упражнение `match` с 5 словами и 5 вариантами, без верных пар в ответе; тема `skill:match`; запись в `exercises` с пустым `sentence_id`); мало слов или нет словаря → вид не выдаётся, остальные работают; ответ: все верные (счёт 1,0, `correct: true`), часть верных (дробный счёт, `correct: false`, `pairs` с разбором), параллельные ответы и повтор по ключу (тот же результат, рейтинги не меняются), другой ключ 409, чужое упражнение 404, неверный формат 400 с русским текстом; ошибка не открывается в банке; событие содержит `correctPairs`.
- [ ] **Step 3:** Реализовать: `matchCandidate` в `exercises_pick.go` (слова как у `formWords`, плюс запрос переводов набором, значения сгруппированы по паре «лемма + часть речи», затем `BuildMatch`); регистрация темы в `availableTopics` через `exercises.Unlocked` и проверку, что набор собирается; ветка `match` в `saveExercise` (payload с `words` и `options`, ответ с `pairs`); ветка `match` в `answerTx` (проверка `CheckMatch`, дробный счёт для Glicko, без `recordMistake`, повтор по ключу возвращает тот же `pairs` из события); ограничение длины `given` для `match` 400.
- [ ] **Step 4:** Интеграционный тест на Postgres во временной схеме: загрузка образца словаря, лексемы с уровнем и карточки, выдача и ответ `match`, повторный запрос по ключу, параллельные ответы; проверка рейтингов `skill:match` и `level:*`.
- [ ] **Step 5:** `go vet ./... && go test -race ./... && golangci-lint run`; commit вместе с контрактом и сгенерированными файлами: `Сопоставление: контракт, выдача и проверка на сервере`.

### Task 4: Фронтенд, логика и компонент без анимаций

**Files:**
- Create: `web/src/features/exercises/match.ts`, `match.test.ts`, `MatchExercise.tsx`, `MatchExercise.test.tsx`
- Modify: `web/src/api/client.ts` (типы), `web/src/features/exercises/ExerciseCard.tsx` (ветка `match`), `web/e2e/mock-api.ts`

**Interfaces:**
- Consumes: контракт Task 3.
- Produces (`match.ts`, чистые функции): `type MatchState = { selected: number | null; pairs: (number | null)[] }` (индекс слова → индекс варианта); `selectWord(state, i)`, `selectOption(state, j)` (замыкает пару, перебивая прежнюю пару слова или варианта), `unpair(state, i)`, `isComplete(state)`, `toGiven(words, options, state): string` (формат `слово=перевод;…`), `pairColor(index): 0..4`, `optionLabel(j): 'a'…'e'`.

- [ ] **Step 1:** Тесты `match.ts` (RED): выбор слова и варианта соединяет; повторное нажатие на выбранное слово снимает выбор; выбор уже занятого варианта переносит его (старая пара рвётся); разрыв пары; `isComplete` только при 5 парах; `toGiven` порядок и формат; клавиатурные буквы a–e.
- [ ] **Step 2:** Реализовать `match.ts`; зелёные.
- [ ] **Step 3:** Тесты компонента (RED): отрисовка 5 слов и 5 вариантов; соединение мышью и клавиатурой (цифра, затем буква); «Проверить» выключена до 5 пар; Backspace рвёт последнюю пару; после ответа у каждой пары значок и слова «верно» или «неверно» (не только цвет), счёт, лисёнок радуется при ≥4 из 5 и думает иначе; ошибка запроса показывается с «Повторить»; повторное нажатие не отправляет два ответа (кнопка блокируется); доступные имена и `aria-pressed` у плиток; живая область объявляет результат.
- [ ] **Step 4:** Реализовать `MatchExercise` (структурно: две колонки, плитки-кнопки, `kbd`-подписи, область результата `role="status"`), подключить в `ExerciseCard` рядом с `order`; `mock-api.ts` получает упражнение `match` и результат. Без анимаций (кроме уже существующих переходов).
- [ ] **Step 5:** `npx tsc -b && npm run lint && npx vitest run`; commit: `Сопоставление: логика пар и компонент`.

### Task 5: Анимации и оформление

**Files:**
- Create: `web/src/features/exercises/MatchThreads.tsx` (SVG-нити), `web/src/features/exercises/matchFx.ts` (чистые функции частиц), тесты к ним
- Modify: `web/src/features/exercises/MatchExercise.tsx`, `web/src/index.css` (ключевые кадры), `DESIGN.md`

**Interfaces:**
- Consumes: `match.ts`, `MatchExercise` из Task 4.
- Produces: `sparkOffsets(seed: number, n: number): {dx: number; dy: number}[]` (n точек на окружности радиуса 12px, детерминированно), `confetti(seed: number, n: number): {x: number; y: number; delay: number; color: 0|1|2|3|4}[]`, `threadPath(from: Point, to: Point): string` (ступенчатый путь клеток по 4px).

- [ ] **Step 1:** Тесты частиц и пути (RED): `sparkOffsets` детерминирован, 4 точки на радиусе 12px, разный `seed` меняет порядок; `confetti` 12 штук внутри области, задержки не больше 200 мс, цвета из 5; `threadPath` ломаная из горизонтальных и вертикальных отрезков кратных 4px, начинается и кончается в заданных точках. Реализовать, зелёные.
- [ ] **Step 2:** Навыки по порядку: `animate` (решения: что анимировать, свойства, кривые, длительности), затем реализация по списку из спека (вход со ступенчатой задержкой 40 мс, подъём выбранной плитки, нить 220 мс `steps(8)`, разрыв 140 мс, проверка по очереди с интервалом 90 мс, искры 280 мс, тряска ±3px 240 мс `steps(4)`, счёт ступенями 300 мс, конфетти 700 мс и подпрыгивание лисёнка при 5 из 5, подсветка рамки 400 мс); только `transform` и `opacity`.
- [ ] **Step 3:** Условия: анимации включаются только для действий мыши и касания (клавиатурные идут без них; признак источника ввода запоминается при событии), при `prefers-reduced-motion` в CSS и в JS-логике времени (последовательная проверка без задержек) остаются цвет, рамка и значок; Vitest: компонент без `animate`-классов при клавиатурном вводе и при `matchMedia('(prefers-reduced-motion: reduce)')`; результат доступен читалке сразу, не после анимации.
- [ ] **Step 4:** Playwright (`web/e2e/match.spec.ts`): весь путь мышью и клавиатурой, оба результата (5 из 5 и 3 из 5), светлая и тёмная темы, 1440/1024/390 px, без горизонтальной прокрутки, снимки в `web/test-results/screens/`; `reducedMotion: 'reduce'` проверяет отсутствие анимаций через `getAnimations().length === 0`.
- [ ] **Step 5:** Навыки `emil-design-eng`, `impeccable` (сверка с `DESIGN.md`) и `web-design-guidelines` по `MatchExercise.tsx` и `MatchThreads.tsx`; исправить найденное; `DESIGN.md` получает раздел «Сопоставление» (плитки, нити, цвета пар, анимации и условия).
- [ ] **Step 6:** Все проверки фронтенда (`tsc`, `lint`, `vitest`, `build`, `e2e`); commit: `Сопоставление: анимации и оформление`.

### Task 6: Живая проверка и документы

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1:** Запустить сервер с новым кодом на порту 8090 (не трогать работающий на 8080) на локальной базе, войти пользователем для проверки (пароль не записывать), вызвать `GET /exercises/next` до выдачи `match` или убедиться, что набор собирается; ответить через `POST /exercises/answer`; проверить в базе строку `exercises` (`kind='match'`, `sentence_id IS NULL`), событие `exercise.answered`, рейтинг `skill:match`. Остановить свой сервер по номеру процесса (не `pkill`).
- [ ] **Step 2:** `CLAUDE.md`: абзац про «Сопоставление» (что сделано, что проверено вживую и что нет, ограничения: нет банка ошибок, шум словаря) и строка в «Ещё нет».
- [ ] **Step 3:** Полный прогон: `go vet ./... && go test -race ./... && golangci-lint run`, из `web/`: `npx tsc -b`, `npm run lint`, `npx vitest run`, `npm run build`, `npm run e2e`.
- [ ] **Step 4:** Commit: `Сопоставление: документация и живая проверка`. Дальше итоговое ревью всей ветки (агент `reviewer`), одна правка по результатам, пуш и PR (база `feat/source-freedict`).
