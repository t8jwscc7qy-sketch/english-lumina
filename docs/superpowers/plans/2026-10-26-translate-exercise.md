# Упражнение «Перевод» Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Новый вид упражнения `translate`: пользователь печатает английский перевод русского предложения; проверка терпимая к регистру, пунктуации, сокращениям и опечатке в одну букву; работа над ошибками как у «порядка слов».

**Architecture:** Чистая проверка в `internal/exercises/translate.go`; выдача копирует `orderCandidate`; `answerTx` получает ветку `translate`; миграция `00013` расширяет `exercises_kind_check`; контракт `api/openapi.yaml` → кодогенерация; фронтенд — компонент `TranslateExercise` внутри `ExerciseCard`.

**Tech Stack:** Go (sqlc не нужен: новых запросов нет), oapi-codegen, React + TypeScript, Vitest, Playwright. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-26-translate-exercise-design.md`

## Global Constraints

- Не добавлять зависимости. Миграцию `00013` писать, но **к настоящей базе не применять** (нужны `pg_dump` и «да» пользователя); проверять в временной схеме (`testdb`).
- Контракт меняется сначала в `api/openapi.yaml`, потом `go generate ./internal/api` и `npm run gen:api` (в `web/`); сгенерированное руками не править.
- `maxAnswerLen = 200`; предложения 4–9 слов, не длиннее 80 символов; допустимые символы `[A-Za-z0-9 ,.!?'’-]`.
- Опечатка: расстояние Дамерау–Левенштейна 1 по всей нормализованной фразе, длина варианта не меньше 12 символов; `Typo` засчитывается верным (счёт Glicko 1), ошибку не открывает.
- Раскрытие сокращений только однозначных: `won't`, `can't`, `cannot`, `shan't`, `n't`, `'m`, `'re`, `'ve`, `'ll`; `'s` и `'d` не раскрываются.
- Интерфейс по-русски, движение только для мыши и касания и без «уменьшить движение»; тексты из базы только текстом (`dangerouslySetInnerHTML` запрещён).
- Перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`, в `web/` — `npx vitest run && npx tsc -b && npm run lint`.

## Review Focus

- Ответ из пробелов, пустой, длиннее 200 символов, с эмодзи и юникодом: `Wrong`, без паники, без 500.
- Короткие фразы: «He is here» и «We is here» (10 символов) не должны засчитываться как опечатка.
- Опечатка не открывает ошибку и не ломает серию; повтор запроса по ключу отдаёт тот же `nearMiss`.
- Копия ошибки `translate` возвращается без перемешивания и со всеми допустимыми ответами.
- Предложение с кавычками, `;`, `…`, двоеточием, цифрой-тире или 2/10 словами не выдаётся.
- Упражнение без альтернатив: `accepted` пуст, а не `null`.

---

### Task 1: Проверка и сборка перевода

**Files:**
- Create: `internal/exercises/translate.go`
- Test: `internal/exercises/translate_test.go`

**Interfaces:**
- Consumes: `maxAnswerLen` (`internal/exercises/gap.go`).
- Produces:
  - `type Translate struct{ Answers []string }`
  - `func BuildTranslate(sentence string, alternatives []string) (Translate, bool)`
  - `type TranslateResult int; const (TranslateWrong TranslateResult = iota; TranslateTypo; TranslateExact)`
  - `type TranslateVerdict struct{ Result TranslateResult; Matched string }`
  - `func CheckTranslate(answers []string, given string) TranslateVerdict`
  - `func TranslateTopics(level string) []string` (`skill:translate`, плюс `level:<L>` при известном уровне)
  - `func normTranslate(s string) string` (внутренняя)

- [ ] **Step 1: Написать падающие тесты** в `translate_test.go` (таблицы, имена рядом):
  - `TestCheckTranslateExact`: `answers=["I don't know."]`; `given` = `"i dont know"` → `Wrong` (без апострофа это другое слово), `"I do not know"` → `Exact`, `"  I  DON’T know!  "` → `Exact`, `"I don't, know"` → `Exact`; `answers=["I'm tired."]`, `"I am tired"` → `Exact`; `answers=["He can't swim."]`, `"he cannot swim"` → `Exact`; `answers=["She won't come."]`, `"she will not come"` → `Exact`.
  - `TestCheckTranslateAmbiguousContractionsStay`: `answers=["It's raining."]`, `"it is raining"` → `Wrong`; `answers=["It's raining.","It is raining."]`, `"It is raining"` → `Exact`, `Matched == "It is raining."`.
  - `TestCheckTranslateTypo`: `answers=["We eat red apples every morning."]`; `"we eat red aples every morning"` (удаление) → `Typo`, `Matched` = исходный вариант; вставка, замена, перестановка соседних букв (`"we eat red appels every morning"`) → `Typo`; две опечатки → `Wrong`.
  - `TestCheckTranslateTypoNeedsLongPhrase`: `answers=["He is here."]` (в нормализованном виде 10 символов), `"we is here"` → `Wrong`; `answers=["I like my small house."]` (≥12), одна опечатка → `Typo`; граница: вариант из 11 символов → `Wrong`, из 12 → `Typo`.
  - `TestCheckTranslatePrefersExactOverTypo`: `answers=["I read books every day.","I read book every day."]`, `given` равен второму → `Exact`, `Matched` второй.
  - `TestCheckTranslateEmptyAndLong`: `""`, `"   "`, 201 символ, `"🙂🙂🙂"` → `Wrong`, без паники.
  - `TestBuildTranslate`: исходное и подходящая альтернатива (другой порядок слов с тем же переводом) → `Answers=[исходное, альтернатива]`; дубликат после нормализации отбрасывается; границы: 3 слова — ok, 2 — нет, 9 — ok, 10 — нет, 81 символ — нет; отказ на `"He said "hi" to me."`, `"Tom, wait; stop."`, `"Is it (really) so?"`, `"Wait... what?"`, `"Café au lait please."`; альтернатива с недопустимым символом пропускается, не ломая остальные.
  - `TestTranslateTopics`: `""` → `["skill:translate"]`, `"B1"` → `["skill:translate","level:B1"]`.

- [ ] **Step 2: Запустить** `go test ./internal/exercises -run 'Translate' -v` → FAIL (не компилируется: нет символов).

- [ ] **Step 3: Реализовать** по сигнатурам выше. `normTranslate`: нижний регистр → `’`/`‘` в `'` → раскрытие сокращений по таблице в правильном порядке (`won't`, `can't`/`cannot`, `shan't` раньше общего `n't`) → удалить `,` → схлопнуть пробелы → отрезать завершающие `.!? `. Расстояние Дамерау–Левенштейна (оптимальное выравнивание строк, на рунах) — отдельная приватная функция `editDistance1(a, b string) bool` (быстрый выход, если длины рун отличаются больше чем на 1). `CheckTranslate`: сначала поиск `Exact` по всем вариантам, затем `Typo` (варианты не короче 12 рун после нормализации); пустое и длиннее `maxAnswerLen` рун → `Wrong`. `BuildTranslate` проверяет предложение и каждую альтернативу одной приватной функцией `translatable(s string) bool` (символы, длина, 4–9 слов).

- [ ] **Step 4: Запустить** `go test -race ./internal/exercises/...` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/exercises/translate.go internal/exercises/translate_test.go
git commit -m "Перевод: проверка с терпимостью к мелочам и опечатке в одну букву"
```

---

### Task 2: Миграция и контракт

**Files:**
- Create: `migrations/00013_exercises_translate.sql`
- Modify: `api/openapi.yaml` (`Exercise.kind`, `Mistake.kind` описание, `AnswerResult`: `nearMiss`, `accepted`)
- Regenerate: `internal/api` (`go generate ./internal/api`), `web/src/api/*` (`npm run gen:api`)
- Test: интеграционный тест миграции — рядом с тестом `00012` (найти: `grep -rn "00012\|exercises_kind_check" internal/testdb/ internal/httpapi/integration_test.go`), по его образцу

**Interfaces:**
- Produces: `api.Translate` (значение `ExerciseKind`), `api.AnswerResult.NearMiss *string`, `api.AnswerResult.Accepted *[]string`; TypeScript-типы с теми же полями.

- [ ] **Step 1: Тест миграции (красный).** По образцу теста `00012`: в временной схеме `INSERT` упражнения с `kind='translate'` проходит; откат (`goose down` одной миграции или SQL из секции Down) падает с понятной ошибкой, пока есть строка `translate`, и проходит после её удаления.

- [ ] **Step 2: Запустить** тест → FAIL (kind не разрешён).

- [ ] **Step 3: Написать миграцию** по образцу `00012`: Up — пересоздать `exercises_kind_check` с `('gap','order','form','match','translate')`; Down — `DO $$` с `RAISE EXCEPTION 'откат 00013 невозможен: есть упражнения вида translate, удалите их'`, затем вернуть прежнюю проверку. Комментарий «к настоящей базе применять только после pg_dump».

- [ ] **Step 4: Контракт.** В `openapi.yaml`: `Exercise.kind.enum` + `translate` (описание: русское предложение в `prompt`, `choices` пуст, `tiles` нет); `AnswerResult.nearMiss` (string, «только у translate, если ответ принят с опечаткой: вариант, с которым совпало») и `AnswerResult.accepted` (массив строк, «только у translate: другие допустимые переводы, до трёх»); описание `Mistake.kind` — `gap, order, form или translate`. Выполнить `go generate ./internal/api` и в `web/` `npm run gen:api`; `go build ./...`.

- [ ] **Step 5: Запустить** тест миграции и `go test -race ./internal/testdb/... ./internal/api/...` → PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/00013_exercises_translate.sql api/openapi.yaml internal/api web/src/api internal/testdb
git commit -m "Перевод: миграция 00013 и контракт (kind translate, nearMiss, accepted)"
```

---

### Task 3: Выдача, ответ и повтор ошибок на сервере

**Files:**
- Modify: `internal/httpapi/exercises.go` (`translateTopic`, `availableTopics`, выбор кандидата, `saveExercise`), `internal/httpapi/exercises_pick.go` (`translateCandidate`), `internal/httpapi/exercises_answer.go` (ветка `translate`, повтор по ключу), `internal/httpapi/mistakes.go` (ветка `translate` в `repeatExercise`, `exerciseView`)
- Test: `internal/httpapi/exercises_translate_test.go` (новый), правка существующих тестов, которые перечисляют темы/виды

**Interfaces:**
- Consumes: `exercises.BuildTranslate/CheckTranslate/TranslateTopics`, `orderMaxLevel`, `orderCandidate` (образец), `api.Translate`, `api.AnswerResult.NearMiss/Accepted` (Task 2).
- Produces: `const translateTopic = "skill:translate"`; `func (s *authServer) translateCandidate(ctx, rng *idRange, maxLevel string, used map[int64]bool) (gapCandidate, bool, error)`; поле `gapCandidate.translate *exercises.Translate`.

- [ ] **Step 1: Написать падающие тесты** (образцы: `exercises_order_test.go`, хелперы `newExerciseEnv`, `login`, `getNext`, `answerBody`, `answerEvents`, `poolSentence`, `knownAll`, `orderFirst`; добавить `translateFirst(e)` — такой же, но с рейтингом `skill:order` и `skill:translate`, чтобы выбиралась `translate`):
  - `TestNextTranslateTopic`: упражнение `kind=translate`, `prompt` — русский перевод, `choices` пуст, `tiles` и ответа в теле нет, тема `skill:translate`, в БД `topics=["skill:translate","level:A1"]`, `answer.answers[0]` — исходное предложение.
  - `TestNextTranslateSkipsUnsuitable`: предложение с кавычками или 10 словами не выдаётся (`404 no_exercise`, если других нет).
  - `TestAnswerTranslateExact`: ответ другим регистром без точки → `correct=true`, `nearMiss` нет, `accepted` — `[]` (не `null`), рейтинг вырос, событие `correct=true`.
  - `TestAnswerTranslateAlternative`: у пары есть альтернативный перевод (через `addPair`); ответ альтернативой верный, `accepted` содержит другие допустимые варианты (до трёх), без основного.
  - `TestAnswerTranslateTypo`: ответ с опечаткой → `correct=true`, `nearMiss` = совпавший вариант, ошибка в банке **не** открыта (`mistakes` пуст, нет события `mistake.opened`), счёт Glicko 1; повтор того же запроса по ключу отдаёт тот же `nearMiss` и не меняет рейтинг.
  - `TestAnswerTranslateWrong`: `correct=false`, `answer` — основной вариант, ошибка открыта (событие `mistake.opened`), `accepted` показан.
  - `TestAnswerTranslateGivenTooLongIsWrong`: 201 символ → `correct=false`, не 500.
  - `TestRepeatTranslateMistake`: после неверного ответа и сдвига времени повтор выдаётся (`repeat=true`), `prompt` тот же, ответов нет в теле; после двух верных ошибка закрыта (по образцу тестов `order` в `mistakes_test.go`).

- [ ] **Step 2: Запустить** `go test ./internal/httpapi -run Translate -v` → FAIL.

- [ ] **Step 3: Реализовать.**
  - `translateTopic` в `availableTopics` (после `orderTopic`, всегда доступна); `case topic == translateTopic:` в выборе; `translateCandidate` по образцу `orderCandidate` с `BuildTranslate(p.row.Text, texts)` и отбором `BuildTranslate(row.Text, nil)` на первом проходе; темы `exercises.TranslateTopics`.
  - `saveExercise`: для `c.translate != nil` — `kind = api.Translate`, `prompt = c.translation` (русское), `translation` пустой, `choices = []`, `answers = c.translate.Answers`.
  - `answerTx`: для `translate` `v := exercises.CheckTranslate(answer.Answers, req.Given)`, `correct = v.Result != TranslateWrong`, `score = 1` для верных; событие хранит `nearMiss` при `Typo`; `recordMistake` вызывается только если `!correct` (проверить, что он уже так работает, иначе защитить условием для `translate`); `result.NearMiss` и `result.Accepted` (`answer.Answers` без `Answers[0]` и без `Matched`, не больше трёх, всегда не nil) выставляются и при повторе по ключу (читать `nearMiss` из события).
  - `repeatExercise`: `case string(api.Translate):` — только проверка `len(a.Answers) > 0`, копия без изменений.
  - Привести в порядок места, где виды перечисляются (`exerciseView`, `MistakeResult`, страницы-обработчики ошибок): `grep -n "api.Order" internal/httpapi/*.go`.

- [ ] **Step 4: Запустить** `go vet ./... && go test -race ./internal/httpapi/... ./internal/exercises/...` → PASS (в том числе все старые тесты).

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "Перевод: выдача, проверка ответа и повтор ошибок"
```

---

### Task 4: Экран «Перевод»

**Files:**
- Create: `web/src/features/exercises/TranslateExercise.tsx`, `TranslateExercise.test.tsx`, `translateDiff.ts`, `translateDiff.test.ts`
- Modify: `web/src/features/exercises/ExerciseCard.tsx` (ветка `translate`), `topics.ts` (`'skill:translate': 'Перевод'`), `web/src/features/mistakes/MistakesPage.tsx` (`lang` для `translate` русский), `exercises.ts` при необходимости; стили в `web/src/index.css`, если нужны (по `DESIGN.md`)
- Skills: **перед написанием компонента вызвать `impeccable` (основной), `animate` и `emil-design-eng`; после — `web-design-guidelines` и `break-ui`** (требование CLAUDE.md для фронтенда).

**Interfaces:**
- Consumes: `Exercise.kind === 'translate'`, `AnswerResult.nearMiss/accepted/correct/answer` (сгенерированные типы), `lastInput`, `prefersReducedMotion`, `sparkOffsets` из `matchFx.ts`, `Fox` / `FoxNote`.
- Produces:
  - `export function diffOneEdit(given: string, expected: string): { start: number; end: number } | null` — границы отличающегося фрагмента в `given` (для подсветки `<mark>`), `null`, если расстояние не 1.
  - `<TranslateExercise frozen pending result onSubmit restoreFocus />`; `result: AnswerResult | undefined`; `onSubmit(text: string)`.

- [ ] **Step 1: Падающие тесты.**
  - `translateDiff.test.ts`: замена, вставка, удаление, перестановка соседних букв, одинаковые строки (`null`), две правки (`null`), юникод, пустые строки.
  - `TranslateExercise.test.tsx` (образец — `OrderExercise.test.tsx`): подпись «Твой перевод» связана с полем; `lang="en"`, `spellCheck=false`, `autoCapitalize="off"`; кнопка «Проверить» недоступна при пустом/пробельном поле и включается при тексте; `Enter` отправляет обрезанный текст, `Shift+Enter` не отправляет; `Esc` очищает; после `result` поле только для чтения; три результата в живом регионе: «Верно» (+ «Так тоже можно: …» при `accepted`), «Почти верно: опечатка» (+ «Правильно: …» и `<mark>` вокруг отличающейся буквы), «Неверно» («Верный ответ: …»); поле не принимает ввод при `frozen`; `data-motion="off"` при источнике ввода «клавиатура» и при «уменьшить движение», `"on"` при мыши; подсказка клавиш скрыта для скринридера (`aria-hidden`), `aria-keyshortcuts` на поле.
  - В `ExerciseCard.test.tsx`: для `kind='translate'` русская фраза с `lang="ru"` и подписью «Переведи на английский», ярлык «Повтор ошибки» при `repeat`, «Дальше» после ответа, `Пробел`/`Enter` ведут дальше только когда фокус не в поле.

- [ ] **Step 2: Запустить** `cd web && npx vitest run src/features/exercises` → FAIL.

- [ ] **Step 3: Реализовать** по разделу «Экран» спека: цитата с полосой sun 4px, лисёнок 48px, `textarea` (2 строки, растёт до 4), рамка `pixel-frame` с `--frame`, кнопка «Проверить» (primary) → после ответа «Дальше» остаётся за `ExerciseCard`, результат в `role="status"`, подсветка буквы через `diffOneEdit`, тряска/искры/прыжок лисёнка по правилам `data-motion` (классы и ключевые кадры взять из `Match*`/`index.css`, новых токенов нет), `sr-only` тексты. `ExerciseCard`: `isTranslate`, не считать `manual`, фокус на поле до ответа и на карточке после, русская фраза в карточке как у `order`.

- [ ] **Step 4: Запустить** `npx vitest run && npx tsc -b && npm run lint` → PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "Перевод: экран с вводом, разбором опечатки и анимациями"
```

---

### Task 5: Сквозные тесты, документация, проверка

**Files:**
- Create: `web/e2e/translate.spec.ts`
- Modify: `web/e2e/mock-api.ts` (если нужен вид `translate`), `DESIGN.md` (раздел «Перевод»), `CLAUDE.md` (статус, известные ограничения, команды без изменений), спек (только при расхождениях)

- [ ] **Step 1: Playwright с подменой API** (образец — `exercises-order.spec.ts`, `match.spec.ts`): верный (Enter), с опечаткой (подсвеченная буква, «Правильно: …»), неверный («Верный ответ», «Вернётся завтра»), повтор ошибки; 390px; светлая и тёмная темы; «уменьшить движение»; клавиатура и мышь. Запуск: `cd web && npm run e2e -- translate`.

- [ ] **Step 2: Живая проверка на сервере** (без записи в базу пользователя): временная схема/песочница по образцу «Маршрута дня» (`english_pass_sandbox`, миграции до `00013` применяются только в песочнице), выдача `translate` и три ответа (верный, опечатка, неверный) через настоящий Go-сервер; события в журнале; временные файлы удалить.

- [ ] **Step 3: Документация.** `DESIGN.md` — раздел «Перевод» (структура экрана, состояния, движение, доступность, без новых токенов); `CLAUDE.md` — абзац «Упражнение «Перевод»» по образцу соседних (вид, тема, миграция `00013` и ПОРЯДОК ВНЕДРЕНИЯ: `pg_dump` → `goose up` → сервер, что проверено и что НЕТ, известные ограничения из спека).

- [ ] **Step 4: Полная проверка и commit**

Run: `go vet ./... && go test -race ./... && golangci-lint run` и в `web/` `npx vitest run && npx tsc -b && npm run lint && npm run e2e` → всё чисто.

```bash
git add web/e2e DESIGN.md CLAUDE.md docs
git commit -m "Перевод: сквозные тесты и документация"
```

---

## Self-review

- Покрытие спека: проверка и сборка (Task 1), миграция и контракт (Task 2), выдача, ответ, опечатка, `accepted`, банк ошибок (Task 3), экран, доступность, движение (Task 4), тесты, документация, живая проверка (Task 5). «Страница Ошибки» — Task 4 (`MistakesPage`). «Затронуто: Перевод» — `topics.ts` в Task 4.
- Имена: `translateTopic`, `translateCandidate`, `exercises.BuildTranslate/CheckTranslate/TranslateVerdict/TranslateTopics`, `api.Translate`, `NearMiss`, `Accepted`, `diffOneEdit`, `TranslateExercise` совпадают во всех задачах.
- Миграция к настоящей базе не применяется в плане: это решение пользователя; в итоговом сообщении напомнить порядок (`pg_dump` → `goose up` → сервер).
