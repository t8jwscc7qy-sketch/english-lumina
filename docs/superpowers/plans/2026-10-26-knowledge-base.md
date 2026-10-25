# Каркас базы знаний `kb` и источник Kaikki Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Собрать в таблицах с префиксом `kb_` лучшие переводы, произношение, формы и синонимы из Kaikki (Викисловарь) и уже загруженных источников, отдельно от данных пользователя, и подключить их к «Словам» и «Сопоставлению».

**Architecture:** Три шага: сырьё (файл в `.cache`), подготовка (`cmd/kb-prepare`: потоковый отбор нужных записей в компактный файл), сборка (`cmd/kb-build`: чистые правила слияния `internal/kb`, запись в `kb` одной транзакцией, отчёт качества с порогами). Потребители читают `kb`, при пустой `kb` работает прежний путь.

**Tech Stack:** Go, pgx (`CopyFrom`) + sqlc, goose, oapi-codegen, React. Новых зависимостей нет (gzip и JSON из стандартной библиотеки).

**Spec:** `docs/superpowers/specs/2026-10-26-knowledge-base-design.md`

## Global Constraints

- Без новых зависимостей. Таблицы `kb_*` создаётся миграцией `00014` (номер сверить с последней в `migrations/` на момент работы); к настоящей базе миграцию **не применять без `pg_dump` и «да» пользователя**; проверка — во временной схеме (`testdb`).
- SQL только в `queries/` через sqlc; генерированное руками не править; контракт API меняется сначала в `api/openapi.yaml`, потом `go generate ./internal/api` и `npm run gen:api`.
- Данные пользователя (`cards`, `exercises`, `mistakes`, `events`, `settings`, `word_learning` и т. п.) таблицы `kb_*` не трогает; на `kb` ссылаются только по `public.lexemes.id`.
- Сборка идёт одной транзакцией через `DELETE` и вставку, **без `TRUNCATE`** (читатели видят прежние данные до фиксации).
- Логика слияния — чистые функции в `internal/kb` с юнит-тестами; константы оценок в коде.
- Файл Kaikki: `https://kaikki.org/dictionary/raw-wiktextract-data.jsonl.gz` (≈ 2,9 ГБ), HTTPS, SHA-256 закрепляется в коде после первой загрузки; изменившийся файл — ошибка суммы, не молчаливое обновление. Кэш `.cache/` вне git.
- Лицензия источника: CC BY-SA и GFDL (как у Викисловаря), атрибуция «Wiktionary contributors, extracted by Wiktextract (Tatu Ylonen), kaikki.org; Tatu Ylonen, Wiktextract: Wiktionary as Machine-Readable Structured Data, LREC 2022»; в этом шаге звук хранится только ссылкой.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`, в `web/` — `npx vitest run && npx tsc -b && npm run lint`.

## Review Focus

- Слово без записи в Kaikki или без русских переводов: сборка не падает, поле пусто, «Слова» и «Сопоставление» работают.
- Лемма с вариантами (`colour/color`, `a.m.`): каждый вариант ищется отдельно, ни одна запись не теряется и не дублируется.
- Переводы с пометками `archaic`, `obsolete`, `dated`, `vulgar`, `offensive` не выбираются при наличии обычного.
- Повторная сборка с теми же входными файлами даёт те же строки (порядок, оценки, выбор).
- Сборка с просевшим покрытием откатывается и оставляет прежние данные; параллельный читатель во время сборки видит прежние данные.
- Пустая `kb` (сборки ещё не было): «Слова» и «Сопоставление» работают как раньше, без ошибки 500.

---

### Task 1: Таблицы `kb_*` и запросы

**Files:**
- Create: `migrations/00014_kb_schema.sql`, `queries/kb.sql`
- Regenerate: `internal/db` (`sqlc generate`)
- Test: `internal/testdb/kb_test.go`

**Interfaces:**
- Produces: таблицы `kb_*` с таблицами `build`, `sense`, `translation`, `pronunciation`, `form`, `relation`, `lexeme_stat`, `teaching_card` (колонки как в спеке, раздел «Таблицы `kb_*`»); запросы sqlc `KbDeleteAll :exec` (удаляет строки всех таблиц `kb` кроме `build`), `KbInsertBuild`/`KbFinishBuild`, `KbGetTeachingCards :many` (по массиву `lexeme_id`), `KbListChosenTranslations :many` (по массиву `lexeme_id`, язык), `KbLatestBuild :one`. Пакетные вставки делает сборка через `pgx.CopyFrom` (запросов sqlc на них нет).

- [ ] **Step 1: Тест (красный)** `TestKbSchema`: после `Pool(t)` существует таблицы `kb_*` со всеми таблицами; в `kb_translation` вставка проходит для `lang='ru'`, нарушение `chosen` без `lexeme_id` отклоняется; `Down` схему удаляет (`SELECT to_regnamespace('kb')` пусто), `Up` создаёт заново; данные `public.cards` и `public.lexemes` после `Down` на месте.
- [ ] **Step 2:** `go test ./internal/testdb -run Kb` → FAIL (схемы нет).
- [ ] **Step 3:** Написать миграцию (внешние ключи на `public.lexemes(id) ON DELETE CASCADE`, индексы: `kb_form(form)`, `kb_translation(lexeme_id) WHERE chosen`, `kb_pronunciation(lexeme_id)`, `kb_relation(lexeme_id)`), Down — `DROP SCHEMA kb CASCADE`. Добавить `queries/kb.sql`; `sqlc generate`.
- [ ] **Step 4:** `go test ./internal/testdb/...` → PASS.
- [ ] **Step 5: Commit** `git add migrations queries internal/db internal/testdb && git commit -m "kb: схема базы знаний и запросы"`.

---

### Task 2: Разбор записей Kaikki

**Files:**
- Create: `internal/kb/kaikki.go`, `internal/kb/kaikki_test.go`, `internal/kb/testdata/kaikki-sample.jsonl`

**Interfaces:**
- Produces:
  - типы `Entry{Word, Pos, LangCode string; Senses []Sense; Sounds []Sound; Forms []Form; Translations []Translation; Synonyms, Antonyms []string}`, `Sense{Glosses, Tags []string; Translations []Translation}`, `Translation{Code, Word, Sense string; Tags []string}`, `Sound{IPA, Audio string; Tags []string}`, `Form{Form string; Tags []string}`;
  - `func ParseEntry(line []byte) (Entry, error)` — разбор одной строки JSONL (поля `word`, `pos`, `lang_code`, `senses[].glosses/tags`, `sounds[].ipa/audio/tags`, `forms[].form/tags`, `translations[].code/word/sense/tags` на уровне записи и значения, `synonyms[].word`, `antonyms[].word`);
  - `func Wanted(line []byte, lemmas map[string]bool) bool` — дешёвая проверка до полного разбора: язык английский (и `"lang_code": "en"`, и `"lang_code":"en"`) и слово из `lemmas`.

- [ ] **Step 1: Тесты (красные)** на фикстуре из ~20 строк (записи: `dad` с несколькими значениями и русскими переводами по `sense` (в том числе `батюшка` с пометкой `archaic` у значения), `clothes`, `blow` (с пометкой `vulgar`), слово с британским и американским IPA, запись `form-of`, запись другого языка, запись не из `lemmas`): `TestParseEntry` (значения, пометки, переводы с `sense`, звуки, формы, синонимы), `TestWantedBothSpacings`, `TestWantedSkipsOtherLanguageAndUnknownWord`, `TestParseEntryBrokenJSON` (ошибка, не паника), `TestParseEntryEmptyFields` (нет полей — нулевые значения).
- [ ] **Step 2:** `go test ./internal/kb` → FAIL.
- [ ] **Step 3:** Реализовать разбор на `encoding/json` с локальными структурами; неизвестные поля игнорируются.
- [ ] **Step 4:** `go test -race ./internal/kb` → PASS.
- [ ] **Step 5: Commit** `git commit -m "kb: разбор записей Kaikki"`.

---

### Task 3: Правила слияния

**Files:**
- Create: `internal/kb/merge.go`, `internal/kb/merge_test.go`, `internal/kb/golden.go`

**Interfaces:**
- Consumes: `Entry`, `Translation`, `Sound`, `Form` (Task 2).
- Produces:
  - `type Candidate struct{ Text string; Source string; SenseNo int; Tags []string; Sources []string }`;
  - `type Scored struct{ Candidate; Score float64; Flags []string }`;
  - `func ScoreTranslations(cands []Candidate, ruRank map[string]int) []Scored` — оценка по формуле спека (вес источника Kaikki 3 / FreeDict 1, согласие +2, русская частота +2/+1/−1, первое значение +1, штрафы `archaic/obsolete/dated` −5, `vulgar/offensive` −5, `slang/rare` −2, несколько слов −1, латиница — отбрасывается), результат отсортирован по убыванию оценки, затем по тексту;
  - `func PickTranslations(scored []Scored, max int) []Scored` — до `max` кандидатов с разными текстами и, если известно, разными значениями;
  - `func PickPronunciations(sounds []Sound) []Pron` (`Pron{Accent, IPA, Audio string; Primary bool}`) — основной американский, затем британский; IPA без `/` и `[]`;
  - `func NormalizeLemmaVariants(lemma string) []string` — варианты через `/` по отдельности, без пустых и дублей;
  - `type GoldenCase{Lemma, Pos string; Accept []string}` и `var Golden []GoldenCase` (40 слов, список пишет автор, проверяет пользователь): `dad`, `kid`, `thin`, `clothes`, `blow`, `light`, `table`, `fire`, `bank`, … с допустимыми русскими основными переводами.

- [ ] **Step 1: Тесты (красные)**: `TestScoreTranslationsDropsArchaicWhenCommonExists` (`dad`: кандидаты `батюшка` (Kaikki, `archaic`, ранг 30608), `папа` (Kaikki и FreeDict, ранг 274) → первым `папа`), `TestScoreTranslationsVulgarPenalty` (`blow`), `TestScoreTranslationsAgreementBeatsSingleSource`, `TestScoreTranslationsRejectsLatin`, `TestScoreTranslationsOnlyRareKeepsBest` (слово без частых вариантов не остаётся пустым), `TestScoreTranslationsDeterministic` (одинаковая оценка → порядок по тексту), `TestPickTranslationsDistinctSenses`, `TestPickPronunciations` (US первым, UK вторым, без слэшей, пустое IPA пропускается), `TestNormalizeLemmaVariants` (`colour/color`, `a.m.`, пустое), `TestGoldenListWellFormed` (40 уникальных слов, у каждого хотя бы один допустимый перевод).
- [ ] **Step 2:** `go test ./internal/kb -run 'Score|Pick|Normalize|Golden'` → FAIL.
- [ ] **Step 3:** Реализовать функции (константы оценок именованными константами пакета); `golden.go` — список данными.
- [ ] **Step 4:** `go test -race ./internal/kb` → PASS.
- [ ] **Step 5: Commit** `git commit -m "kb: правила слияния переводов, произношения и вариантов лемм"`.

---

### Task 4: Подготовка Kaikki (`cmd/kb-prepare`)

**Files:**
- Create: `cmd/kb-prepare/main.go`, `cmd/kb-prepare/prepare.go`, `cmd/kb-prepare/prepare_test.go`, `internal/sources/kaikki/kaikki.go` (константы URL, SHA-256, лицензия, атрибуция, `Describe()`)

**Interfaces:**
- Consumes: `kb.Wanted`, `kb.ParseEntry`, `importer.Download`, запросы `UpsertDataSource`, `FinishDataSource`, список лемм из `lexemes`.
- Produces: `func Prepare(ctx context.Context, in io.Reader, lemmas map[string]bool, out io.Writer) (Stats, error)` — читает gzip-поток построчно (буфер строки не меньше 16 МБ), пишет отобранные строки как есть в `out`, возвращает счётчики (прочитано, англ., отобрано); команда `go run ./cmd/kb-prepare kaikki` скачивает файл в `.cache/sources/kaikki/`, проверяет сумму, регистрирует источник (`kaikki-wiktextract`, версия — дата дампа из константы, `RowCount` — число отобранных записей) и пишет `.cache/kb/kaikki-en.jsonl`; путь и хеш входных данных (сумма файла плюс хеш отсортированного списка лемм) записываются в имя файла или соседний `.meta`, чтобы сборка отказала при несовпадении со списком лемм.

- [ ] **Step 1: Тесты (красные)** `TestPrepareFiltersByLanguageAndLemma` (маленький gzip в памяти: английские слова из списка и вне списка, русская запись), `TestPrepareLongLine` (строка 5 МБ не обрывается), `TestPrepareStopsOnCanceledContext`, `TestPrepareCorruptGzipIsError`, `TestPrepareExpandsLemmaVariants` (лемма `colour/color` ищет обе записи), `TestDescribe` (имя, лицензия содержит `CC BY-SA`, атрибуция содержит `Ylonen` и `kaikki.org`).
- [ ] **Step 2:** `go test ./cmd/kb-prepare ./internal/sources/kaikki` → FAIL.
- [ ] **Step 3:** Реализовать. Первый запуск: скачать файл (`curl`), посчитать SHA-256 (`shasum -a 256`), записать в константу — отдельный шаг после реализации (Task 7); до этого константа пуста и команда отказывается скачивать без суммы.
- [ ] **Step 4:** `go test -race ./cmd/kb-prepare ./internal/sources/kaikki` → PASS.
- [ ] **Step 5: Commit** `git commit -m "kb-prepare: потоковый отбор записей Kaikki и регистрация источника"`.

---

### Task 5: Сборка (`cmd/kb-build`)

**Files:**
- Create: `cmd/kb-build/main.go`, `cmd/kb-build/build.go`, `cmd/kb-build/report.go`, тесты рядом, `internal/kb/testdata/` (малая выборка)

**Interfaces:**
- Consumes: `kb.ParseEntry`, `kb.ScoreTranslations`, `PickTranslations`, `PickPronunciations`, `NormalizeLemmaVariants`, `kb.Golden`; запросы Task 1; чтение `public.lexemes`, `lexeme_cefr`, `word_frequencies` (`frequencywords`, `frequencywords-ru`), `word_translations` (`freedict-eng-rus`).
- Produces: `func Build(ctx context.Context, pool *pgxpool.Pool, kaikki io.Reader, opts Options) (Report, error)`; `Report{Levels map[string]LevelStats; Golden GoldenResult; Sample []SampleRow; Flagged int}`; `Options{MinCoverage map[string]float64; MinGolden float64}` (по умолчанию пороги из спека: покрытие переводом A1 0,93, A2 0,91, B1 0,89, B2 0,85, C1 0,71, C2 0,67; контрольный список 0,90); ошибка `ErrBelowThreshold` откатывает транзакцию.

- [ ] **Step 1: Тесты (красные)** на интеграции (`testdb.Pool`): `TestBuildWritesKb` (малые фикстуры: пять слов в `lexemes` с уровнями, Kaikki-выборка, FreeDict-строки, частоты → в `kb` значения, до двух выбранных переводов на слово, произношения, формы, синонимы, `lexeme_stat`, `teaching_card`; у слов без Kaikki-записи карточка строится из FreeDict), `TestBuildIsIdempotent` (две сборки подряд дают одинаковое число и содержимое строк), `TestBuildRollsBackBelowThreshold` (порог не выполнен → `ErrBelowThreshold`, прежние строки `kb` на месте), `TestBuildReaderSeesOldDataDuringBuild` (читатель в другом соединении до фиксации видит прежние данные), `TestBuildDoesNotTouchUserTables` (`cards`, `exercises`, `events` без изменений), `TestBuildEmptyKaikkiKeepsLegacyTranslations`, `TestReportCoverageByLevel`.
- [ ] **Step 2:** `go test ./cmd/kb-build/...` → FAIL.
- [ ] **Step 3:** Реализовать: чтение входов, слияние по леммам (варианты `colour/color` как отдельные поиски), запись через `DELETE` и `CopyFrom` в одной транзакции, отчёт и пороги, запись `kb_build`. Карточка слова (`teaching_card.payload`) — JSON с полями `lemma`, `pos`, `level`, `rank`, `translations` (до двух), `ipa`, `audio`, `forms`, `synonyms`.
- [ ] **Step 4:** `go vet ./... && go test -race ./cmd/kb-build/...` → PASS.
- [ ] **Step 5: Commit** `git commit -m "kb-build: сборка базы знаний, отчёт качества и пороги"`.

---

### Task 6: Потребители («Слова» и «Сопоставление»)

**Files:**
- Modify: `api/openapi.yaml` (в схеме слова поля `ipa`, `translations`, `synonyms`), `internal/httpapi/words.go`, `internal/httpapi/exercises_pick.go` (`matchCandidate`), `queries/kb.sql`, фронтенд «Слова» (`web/src/features/words/*`), `internal/httpapi/auth.go` (интерфейс хранилища)
- Test: `internal/httpapi/words_test.go`, `internal/httpapi/exercises_match_test.go`, `web/src/features/words/*.test.tsx`

**Interfaces:**
- Consumes: запросы `KbGetTeachingCards`, `KbListChosenTranslations` (Task 1).
- Produces: в ответе `GET /api/v1/words` у слова необязательные `ipa: string`, `translations: string[]` (до двух), `synonyms: string[]`; в `matchCandidate`: если для слова есть выбранный перевод в `kb`, он идёт как единственное значение (`PickTranslation` обходится); нет строк в `kb` (сборки не было) — прежний путь.

- [ ] **Step 1: Тесты (красные):** `TestWordsIncludeKbFields` (с поддельным хранилищем, отдающим карточки: `ipa`, два перевода, синонимы), `TestWordsWithoutKbUnchanged` (пустая `kb` → поля нет в ответе, остальное как раньше), `TestMatchPrefersKbTranslation` (в `kb` для слова основной перевод `папа`, в FreeDict `батюшка` → в наборе `папа`), `TestMatchFallsBackWhenKbEmpty`, Vitest: страница «Слова» показывает IPA, переводы и синонимы, а без них не показывает пустых заголовков; тексты выводятся как текст.
- [ ] **Step 2:** `go test ./internal/httpapi -run 'Kb|Words|Match'` и `npx vitest run src/features/words` → FAIL.
- [ ] **Step 3:** Расширить контракт и перегенерировать код (`go generate ./internal/api`, `npm run gen:api`); добавить чтение `kb` в обработчики (один запрос на страницу слов); обновить фронтенд по `DESIGN.md` (строка IPA приглушённым цветом, переводы 600, синонимы текстом через запятую); поддельное хранилище получает те же методы.
- [ ] **Step 4:** `go vet ./... && go test -race ./... && golangci-lint run`, `cd web && npx vitest run && npx tsc -b && npm run lint` → чисто.
- [ ] **Step 5: Commit** `git commit -m "kb: «Слова» и «Сопоставление» читают базу знаний"`.

---

### Task 7: Живой прогон и документация

**Files:**
- Modify: `internal/sources/kaikki/kaikki.go` (SHA-256 и дата дампа), `CLAUDE.md`, `DESIGN.md` (строка «Слова»), страница «Источники» (проверить, что источник виден), спек (если расходится)

- [ ] **Step 1: Подготовка к настоящей базе.** Спросить пользователя «да» на применение миграции; `pg_dump` (без таблиц Tatoeba, как раньше) в `.cache/backups/before-00014.dump`; `goose -dir migrations postgres "$DATABASE_URL" up`.
- [ ] **Step 2: Скачать и закрепить файл:** `curl` в `.cache/sources/kaikki/raw-wiktextract-data.jsonl.gz`, `shasum -a 256`, записать сумму и дату дампа в константы; повторный запуск `go run ./cmd/kb-prepare kaikki` должен пропустить загрузку по сумме.
- [ ] **Step 3: Замеры:** время `kb-prepare` (цель ≤ 20 минут) и `kb-build`, размер `kb` (`pg_total_relation_size`), задержка чтения карточки слова (`EXPLAIN ANALYZE`, цель ≤ 5 мс).
- [ ] **Step 4: Отчёт качества:** покрытие по уровням (не ниже порогов), контрольный список (≥ 90 %), 50 случайных слов с переводами — показать пользователю глазами; записать расхождения и поправить константы оценок (повторить `kb-build`).
- [ ] **Step 5: Живая проверка сервера** (песочница или настоящая база по решению пользователя): «Слова» с IPA и переводами, один набор «Сопоставления»; ничего не писать в журнал пользователя.
- [ ] **Step 6: Документация и проверка:** `CLAUDE.md` (таблицы `kb_*`, команды `go run ./cmd/kb-prepare kaikki`, `go run ./cmd/kb-build`, ПОРЯДОК ВНЕДРЕНИЯ: `pg_dump` → `goose up` → `kb-prepare` → `kb-build` → сервер; что проверено и что НЕТ; известные ограничения); `go vet ./... && go test -race ./... && golangci-lint run` и в `web/` `npx vitest run && npx tsc -b && npm run lint`.
- [ ] **Step 7: Commit** `git commit -m "kb: живой прогон, замеры и документация"`.

---

## Self-review

- Покрытие спека: схема (Task 1), разбор и подготовка (Tasks 2, 4), правила слияния и контрольный список (Task 3), сборка, отчёт и пороги (Task 5), потребители (Task 6), источник и лицензия, живая проверка (Tasks 4, 7).
- Отличие от спека: `cmd/import kaikki` не используется (каркас `importer.Run` требует выдачи строк и считает пустой результат ошибкой), скачивание и регистрацию источника делает `kb-prepare`; спек поправить при исполнении.
- Имена согласованы: `Entry`, `Candidate`, `Scored`, `Build`, `Prepare`, `Report`, `ErrBelowThreshold`, `Golden`.
- Размеры, время и качество реального Kaikki заранее неизвестны: Task 7 меряет и подстраивает константы.
