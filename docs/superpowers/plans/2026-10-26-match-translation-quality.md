# Качество переводов в «Сопоставлении» Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** «Сопоставление» выбирает обычный русский перевод слова, опираясь на частотный список русских слов.

**Architecture:** Новый источник `frequencywords-ru` кладёт `ru_50k` в существующую таблицу `word_frequencies`. Обработчик читает места русских вариантов готовым запросом `ListWordFrequencies` и передаёт их в `PickTranslation`, которая отсеивает редкие варианты и при равенстве выбирает более частый. Без загруженных частот поведение прежнее.

**Tech Stack:** Go, pgx + sqlc (новых запросов нет), importer-каркас проекта. Новых зависимостей и миграций нет.

**Spec:** `docs/superpowers/specs/2026-10-26-match-translation-quality-design.md`

**Отличие от спека:** отдельный запрос `ListRuWordRanks` не нужен: `ListWordFrequencies(source_name, words[])` из `queries/frequencies.sql` уже берёт места слов из последнего завершённого импорта источника с заданным именем. Используем его с именем `frequencywords-ru`.

## Global Constraints

- Не добавлять зависимости; миграций нет; `sqlc generate` не нужен.
- Файл: `https://raw.githubusercontent.com/hermitdave/FrequencyWords/525f9b560de45753a5ea01069454e72e9aa541c6/content/2018/ru/ru_50k.txt`, SHA-256 `6095f507cc167488ec66ada5a85ac50433503a08ad24a07c6eabdf54352c4e7f`, 50 000 строк.
- Имя источника `frequencywords-ru`, версия `2018`, лицензия CC BY-SA 4.0 (текст как у английского модуля, с `ru_50k`).
- `maxRank = 20000` (константа в `internal/exercises/match.go`).
- Пустая карта мест (`len(ranks) == 0`) = прежнее поведение без отсева.
- Контракт API и фронтенд не меняются. Комментарии и тексты ошибок по-русски, как в соседнем коде.
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`.

## Review Focus

- Все варианты хуже порога или без места → слово пропускается (набор без него), а не падает.
- Источник не загружен или ни один вариант не найден в списке → прежний выбор, не ошибка 500.
- Слово из списка в другом написании (`ё`/`е`, регистр, ударение): места ищутся по тому же нормализованному виду, что и выбор.
- Строка файла с латиницей или с возрастающим счётом → понятная ошибка с номером строки, импорт не пишет мусор.
- Повторный импорт того же файла не дублирует строки.

---

### Task 1: `PickTranslation` учитывает места слов

**Files:**
- Modify: `internal/exercises/match.go` (`PickTranslation`, `BuildMatch`, `MatchCandidate`-потребители)
- Modify: все вызовы `PickTranslation`/`BuildMatch` в `internal/exercises/*_test.go` (добавить аргумент `nil`)
- Test: `internal/exercises/match_test.go`

**Interfaces:**
- Consumes: существующие `normalizeRu`, `validRu`.
- Produces:
  - `const maxRuRank = 20000`
  - `func RuCandidates(senses [][]string) []string` — нормализованные (`normalizeRu`) валидные варианты по всем значениям, без повторов, в порядке появления; их места читает обработчик.
  - `func PickTranslation(senses [][]string, ranks map[string]int) (string, bool)`
  - `func BuildMatch(cands []MatchCandidate, ranks map[string]int, seed int64) (Match, bool)`

- [ ] **Step 1: Написать падающие тесты** в `internal/exercises/match_test.go` (таблица, `ranks` по нормализованному написанию):
  - `TestPickTranslationRankBreaksTie`: `senses=[["батюшка","папа"],["батюшка","папа"]]`, `ranks={"папа":274,"батюшка":30608}` → `"папа", true` (без отсева, батюшка > 20000 отброшен).
  - `TestPickTranslationRankTieBreakInsteadOfDictOrder`: `senses=[["редкий","тонкий"],["редкий","тонкий"]]`, `ranks={"редкий":12922,"тонкий":16560}` → `"редкий"` (честно фиксируем ограничение спека: порядок по рангу, а не по смыслу).
  - `TestPickTranslationDropsUnrankedAndRare`: `senses=[["одёжа","одежда"]]`, `ranks={"одежда":4303}` → `"одежда", true`; вариант без места отброшен.
  - `TestPickTranslationAllRareSkipsWord`: `senses=[["батюшка"]]`, `ranks={"батюшка":30608}` → `"", false`.
  - `TestPickTranslationEmptyRanksKeepsOldBehaviour`: те же данные, что в существующем тесте без мест, `ranks=nil` и `ranks=map[string]int{}` дают тот же результат, что до изменения.
  - `TestRuCandidates`: `[["Пап́а","папа"],["Тонкий"]]` → `["папа","тонкий"]` (нормализация, без повторов, порядок появления; точные ожидания сверить с `normalizeRu`).

- [ ] **Step 2: Запустить** `go test ./internal/exercises -run 'PickTranslation|RuCandidates' -v` → FAIL (не компилируется: нет `RuCandidates`, у `PickTranslation` один аргумент).

- [ ] **Step 3: Реализовать** `RuCandidates`, добавить параметр `ranks` в `PickTranslation` и `BuildMatch`. Отсев делается внутри цикла по кандидатам, до подсчёта `any`/`usable` (так правило неоднозначности `usable > 1 && count < 2` работает на оставшихся вариантах): если `len(ranks) > 0`, вариант без места или с местом `> maxRuRank` пропускается. При равенстве числа значений вместо «раньше в словаре» сравнивать место (меньше — лучше), затем прежний порядок (значение, позиция), затем алфавит. Если `len(ranks) == 0`, порядок выбора точно прежний. `BuildMatch` передаёт `ranks` в `PickTranslation`. Обновить существующие вызовы в тестах (`nil`).

- [ ] **Step 4: Запустить** `go test -race ./internal/exercises/...` → PASS (включая старые тесты без изменения ожиданий).

- [ ] **Step 5: Commit**

```bash
git add internal/exercises
git commit -m "Сопоставление: выбор перевода учитывает места слов в списке частот"
```

---

### Task 2: Источник `frequencywords-ru`

**Files:**
- Create: `internal/sources/frequencywordsru/frequencywordsru.go`
- Create: `internal/sources/frequencywordsru/testdata/sample.txt`
- Test: `internal/sources/frequencywordsru/frequencywordsru_test.go`, `frequencywordsru_integration_test.go`
- Modify: `cmd/import/main.go` (рядом со строкой `reg.Register(frequencywords.New())`)

**Interfaces:**
- Consumes: `importer.Source` (`Describe`, `Fetch`, `Parse`, `Load`), `importer.Download`, `db.UpsertWordFrequency` (как в `internal/sources/frequencywords/frequencywords.go`).
- Produces: пакет `frequencywordsru`, `func New() *Source`, внутренний `newSource(url, sha256 string, maxBytes int64, client *http.Client) *Source`, тип `Row{Word string; Count int64; Rank int}`; имя источника `frequencywords-ru`.

- [ ] **Step 1: Написать падающие тесты**, скопировав структуру `internal/sources/frequencywords/*_test.go` и заменив данные на русские:
  - `testdata/sample.txt`: строки вида `я 5081568`, `не 4334804`, `что 3552532`, `ребёнок 10230` в порядке убывания счёта (первые строки реального файла и несколько других).
  - `TestDescribe` (имя `frequencywords-ru`, версия `2018`, лицензия содержит `CC BY-SA 4.0`, атрибуция содержит `ru_50k`).
  - `TestParseSample` (слова, счёт, ранг 1…N), `TestParseCRLFAndBOM`, `TestParseEqualCountsAreAllowed`, `TestParseStopsWhenEmitFails`, `TestParseStopsOnCanceledContext`.
  - `TestParseErrors`: латиница в слове (`cat 100` → ошибка с «строка 1»), слово с цифрой, счёт больше предыдущего, повтор слова, одно поле, нулевой байт, счёт ноль.
  - Интеграционный (по образцу английского, во временной схеме; `sourceFor` с URL `/ru_50k.txt`): после импорта число строк в `word_frequencies` равно числу строк файла; повторный импорт не меняет число строк; `ListWordFrequencies` с `SourceName: "frequencywords-ru"` и словом `ребёнок` отдаёт ожидаемое место.

- [ ] **Step 2: Запустить** `go test ./internal/sources/frequencywordsru/...` → FAIL (пакета нет).

- [ ] **Step 3: Реализовать** пакет как копию английского модуля с константами из Global Constraints; `fileName = "ru_50k-2018.txt"`, `maxFileSize = 10 << 20`. В `parseLine` после проверок английской версии добавить: слово состоит только из кириллических букв и дефиса (`unicode.Is(unicode.Cyrillic, r) || r == '-'`), иначе ошибка `слово не из кириллицы: %q`. Зарегистрировать `reg.Register(frequencywordsru.New())` в `cmd/import/main.go`.

- [ ] **Step 4: Запустить** `go test -race ./internal/sources/frequencywordsru/... ./cmd/import/...` → PASS; `go run ./cmd/import list` показывает `frequencywords-ru`.

- [ ] **Step 5: Commit**

```bash
git add internal/sources/frequencywordsru cmd/import/main.go
git commit -m "Источник frequencywords-ru: частоты русских слов (FrequencyWords ru_50k)"
```

---

### Task 3: Обработчик читает места и передаёт их в выбор

**Files:**
- Modify: `internal/httpapi/exercises_pick.go` (`matchCandidate`, константа `ruFrequencySource = "frequencywords-ru"` рядом с `dictionarySource`)
- Test: `internal/httpapi/exercises_match_test.go` (поддельное хранилище `fakeStore.ListWordFrequencies` уже есть в `words_test.go`; оно игнорирует имя источника и отдаёт только запрошенные слова)

**Interfaces:**
- Consumes: `exercises.RuCandidates`, `exercises.BuildMatch(cands, ranks, seed)` (Task 1); `store.ListWordFrequencies(ctx, db.ListWordFrequenciesParams{SourceName, Words})`.
- Produces: поведение `matchCandidate`: после сборки `senses` собрать объединение `RuCandidates` по всем кандидатам, одним вызовом `ListWordFrequencies` с `SourceName: ruFrequencySource` прочитать места, построить `ranks map[string]int` (слово → `int(Rank)`) и передать в `BuildMatch`. Ошибка запроса возвращается как ошибка (500), как у остальных запросов; пустой результат = прежнее поведение.

- [ ] **Step 1: Написать падающие тесты** в `internal/httpapi/exercises_match_test.go` по образцу соседних (хелпер для переводов — как в строке 41 файла):
  - `TestMatchPrefersFrequentRussianVariant`: у слова `dad` два значения с вариантами «батюшка» и «папа»; в `e.store.frequencies` положить «папа» (место 274) и «батюшка» (30608); в выданном наборе пара `dad` → «папа»; `e.store.lastFreq.SourceName == "frequencywords-ru"`.
  - `TestMatchWithoutRussianFrequenciesKeepsOldChoice`: `e.store.frequencies` пуст → набор выдаётся, как в существующих тестах (регрессия), статус 200.
  - `TestMatchSkipsWordWithOnlyRareTranslation`: у одного из шести слов единственный вариант «батюшка» (место 30608) → слово не попадает в набор, набор из пяти остальных выдаётся.

- [ ] **Step 2: Запустить** `go test ./internal/httpapi -run Match -v` → FAIL.

- [ ] **Step 3: Реализовать** в `matchCandidate`, не меняя остальной логики: собрать уникальные русские варианты, вызвать `ListWordFrequencies` один раз (если вариантов нет — не вызывать), построить `ranks`, передать в `BuildMatch(cands, ranks, s.rnd(1<<62))`.

- [ ] **Step 4: Запустить** `go vet ./... && go test -race ./... && golangci-lint run` → всё чисто.

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "Сопоставление: обработчик читает места русских вариантов и передаёт в выбор"
```

---

### Task 4: Живая проверка и документация

**Files:**
- Modify: `CLAUDE.md` (раздел статуса: источник, правило, результат проверки; в «Команды» добавить `frequencywords-ru` в перечень источников `cmd/import`), `docs/superpowers/specs/2026-10-26-match-translation-quality-design.md` (только строка об отличии: готовый запрос вместо нового)

- [ ] **Step 1: «До».** Запустить сервер как в «Команды» и выписать пары 12 наборов для `GET /api/v1/exercises/next` нельзя без входа под пользователем; вместо этого временный (не коммитить) тест или скрипт на настоящей базе вызывает `PickTranslation(senses, nil)` для слов уровней A1–B2 из `word_translations` и печатает пары для 40 слов, включая `dad`, `kid`, `thin`, `clothes`. Сохранить вывод в `.cache/translation-before.txt` (каталог вне git).

- [ ] **Step 2: Импорт.** `set -a; . ./.env; set +a; go run ./cmd/import frequencywords-ru` → ожидаемо 50 000 строк, повтор запуска ничего не дублирует (`SELECT count(*) FROM word_frequencies wf JOIN data_sources s ON s.id = wf.source_id WHERE s.name = 'frequencywords-ru'` = 50000).

- [ ] **Step 3: «После».** Тот же временный скрипт, но с местами из `ListWordFrequencies`; сохранить в `.cache/translation-after.txt`, сравнить на тех же 40 словах, посчитать, у скольких слов вариант стал нормальным, ухудшился или слово выпало. Результат (с честным списком оставшихся плохих пар, включая `thin`) записать в `CLAUDE.md`. Временный скрипт удалить.

- [ ] **Step 4: Проверка в браузере** под входом пользователя не выполняется самим агентом (нет пароля): в `CLAUDE.md` записать «НЕ проверено вживую в браузере» и дать пользователю шаги: `go run ./cmd/server`, `cd web && npm run dev`, открыть «Упражнения» до выдачи «Сопоставления».

- [ ] **Step 5: Полная проверка и commit**

Run: `go vet ./... && go test -race ./... && golangci-lint run` и в `web/` `npx vitest run && npx tsc -b && npm run lint` → чисто.

```bash
git add CLAUDE.md docs/superpowers
git commit -m "Сопоставление: документация и результаты проверки русских частот"
```

---

## Self-review

- Покрытие спека: источник (Task 2), чтение мест (Task 3, готовый запрос — отличие записано), правило выбора и порог 20 000 (Task 1), поведение без частот (Task 1, Task 3), тесты Parse/интеграции/обработчика (Tasks 1–3), живая проверка до/после (Task 4). Ограничение `thin` зафиксировано тестом `TestPickTranslationRankTieBreakInsteadOfDictOrder` и документацией.
- Согласованность имён: `RuCandidates`, `PickTranslation(senses, ranks)`, `BuildMatch(cands, ranks, seed)`, `ruFrequencySource`, `maxRuRank`, `frequencywordsru` одинаковы во всех задачах.
- Ключи в `ranks` — результат `normalizeRu`; проверено: он убирает только знаки ударения и приводит регистр, `ё` не трогает, поэтому слова находятся в списке (`ребёнок` с `ё`).
