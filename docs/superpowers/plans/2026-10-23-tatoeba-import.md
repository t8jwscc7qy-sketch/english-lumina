# Импорт Tatoeba (пары предложений англ.–рус.) — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `go run ./cmd/import tatoeba` загружает в PostgreSQL 806 586 предложений и 536 106 пар «английское — русское» с авторами; повторный запуск ничего не дублирует.

**Architecture:** Новый модуль `internal/sources/tatoeba` реализует существующий контракт `importer.Source` (каркас не меняется). `Fetch` скачивает zip с проверкой SHA-256 и безопасно достаёт один файл; `Parse` читает его построчно; `Load` через upsert записывает предложения и пары. Таблицы приходят одной миграцией `00003`.

**Tech Stack:** Go 1.27 (`archive/zip`, `bufio`, `regexp`), pgx v5, sqlc, goose. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-23-tatoeba-import-design.md`

## Global Constraints

- Не добавлять зависимости в `go.mod`. SQL только в `queries/`, код sqlc перегенерируется `sqlc generate`, `internal/db/*.go` руками не правится. Миграцию `00003_sentences.sql` пишет навык `db-migration`: обратимая, только добавляет таблицы, применять к реальной базе только с явного «да» пользователя (Task 5).
- Источник закреплён: URL `https://www.manythings.org/anki/rus-eng.zip`, SHA-256 `1534e267976f43ae97e966d2ac9dc1e9128fdd0efdf08eceee21cd88ff20682c`, размер 17 447 329 байт; лимит скачивания 40 МиБ, лимит распаковки `rus.txt` 200 МиБ (файл 89 182 633 байта). Имя `tatoeba`, версия `2026-02-13`. Файлы в `.cache/sources/tatoeba/` вне Git.
- Формат строки `rus.txt`: `английское<TAB>русское<TAB>CC-BY 2.0 (France) Attribution: tatoeba.org #<id> (<автор>) & #<id> (<автор>)`. Пары, где английское длиннее 30 слов (`strings.Fields`), пропускаются.
- Лимиты полей как в схеме: `text` 1–1000 символов, `author` 1–64, `word_count` > 0, языки `en`/`ru`.
- Ожидаемые числа для реального снимка: 536 106 пар, 341 165 английских и 465 421 русских предложений (806 586 всего), 18 пропущенных пар.
- Комментарии и сообщения об ошибках по-русски, в стиле соседнего кода. Коммиты по просьбе пользователя; перед коммитом `go vet ./... && go test -race ./... && golangci-lint run`. Секреты не печатать.

## Review Focus

- `Fetch`: zip с `../rus.txt`, `/rus.txt`, `dir/rus.txt` или без `rus.txt`; `rus.txt` больше лимита распаковки (в том числе когда заголовок zip врёт о размере) — ошибка и никаких файлов-остатков в кэше (Task 3).
- Повтор того же `(lang, tatoeba_id)` с другим текстом или автором — ошибка и откат, а не молчаливая подмена; повтор с тем же текстом — не дубль (Task 4).
- Строка с недопустимым UTF-8, нулевым байтом, лишним полем, чужой лицензией в атрибуции или `\r` на конце: чёткая ошибка с номером строки или корректный разбор, без паники (Task 2).
- Ошибка на середине файла откатывает всё: ни одного предложения и пары из запуска; пустой файл и файл с одним слишком длинным английским предложением — ошибка «нет строк» (Task 4).
- Двойной импорт: те же числа предложений и пар, у общего английского предложения с несколькими переводами одна строка и несколько пар (Task 4).
- Одно и то же предложение от двух источников: остаётся одна строка с `source_id` первого источника (Task 4).

---

### Task 1: Миграция `00003` и запросы sqlc

**Files:**
- Create: `migrations/00003_sentences.sql`, `queries/sentences.sql`
- Modify: `internal/testdb/testdb_test.go` (проверки новых таблиц)
- Regenerate: `internal/db/*.go` (`sqlc generate`)

**Interfaces:**
- Produces (SQL → sqlc): `(*db.Queries).UpsertSentence(ctx, db.UpsertSentenceParams{Lang string, TatoebaID int64, Text, Author string, WordCount int16, SourceID int64}) (db.UpsertSentenceRow, error)` — возвращает `ID int64, Text, Author string` строки, которая лежит в базе под `(lang, tatoeba_id)`: либо только что вставленной, либо уже существовавшей (чтобы вызывающий сравнил текст и автора). `(*db.Queries).UpsertSentencePair(ctx, db.UpsertSentencePairParams{EnID, RuID, SourceID int64}) error` (повтор не ошибка). Точные имена полей сверить по сгенерированному коду.
- Запрос `UpsertSentence` одним оператором: CTE `INSERT … ON CONFLICT (lang, tatoeba_id) DO NOTHING RETURNING id, text, author`, затем `SELECT` из CTE `UNION ALL` `SELECT` существующей строки только если вставки не было, `LIMIT 1`.
- DDL: `sentences(id bigint identity PK, lang text NOT NULL CHECK IN ('en','ru'), tatoeba_id bigint NOT NULL CHECK > 0, text text NOT NULL CHECK length 1–1000, author text NOT NULL CHECK length 1–64, word_count smallint NOT NULL CHECK > 0, source_id bigint NOT NULL REFERENCES data_sources ON DELETE RESTRICT, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (lang, tatoeba_id))` + индекс `sentences (source_id)`; `sentence_pairs(en_id bigint NOT NULL REFERENCES sentences ON DELETE CASCADE, ru_id bigint NOT NULL REFERENCES sentences ON DELETE CASCADE, source_id bigint NOT NULL REFERENCES data_sources ON DELETE RESTRICT, PRIMARY KEY (en_id, ru_id))` + индексы `(ru_id)` и `(source_id)`. Down: `DROP TABLE sentence_pairs; DROP TABLE sentences;` с комментарием, что данные можно загрузить заново командой import.

- [ ] **Step 1: Write the failing tests** в `testdb_test.go`: `allTables` дополняется `sentences` и `sentence_pairs` (цикл up → down → up уже проверяет их наличие); новый `TestSentenceSchemaConstraints`: язык `fr`, `tatoeba_id = 0`, пустой `text`, `text` из 1001 символа, `author` из 65 символов, `word_count = 0` → 23514; повтор `(lang, tatoeba_id)` → 23505; пара с несуществующим `en_id` → 23503; повтор пары `(en_id, ru_id)` → 23505; удаление предложения каскадно удаляет его пары; удаление `data_sources` со ссылающимися предложениями запрещено.
- [ ] **Step 2: Run** `go test ./internal/testdb/...` — Expected: FAIL (таблиц нет).
- [ ] **Step 3: Implement** миграцию и `queries/sentences.sql` по Interfaces; `sqlc generate`; проверить, что у `UpsertSentenceRow` поля не указатели (если sqlc сделал `UNION ALL` nullable — привести запрос так, чтобы типы были `int64`/`string`).
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS и чисто, `TestMigrationsUpDownUp` и интеграционные тесты `httpapi` не пропущены.
- [ ] **Step 5: Commit**: «Схема предложений: sentences, sentence_pairs».

### Task 2: Описание источника и разбор `rus.txt`

**Files:**
- Create: `internal/sources/tatoeba/tatoeba.go`, `internal/sources/tatoeba/tatoeba_test.go`, `internal/sources/tatoeba/testdata/sample.txt`

**Interfaces:**
- Consumes: `importer.Info`, `importer.Source`.
- Produces: `tatoeba.New() *Source`; `type Row struct{ EnID int64; EnText, EnAuthor string; RuID int64; RuText, RuAuthor string }`; константы `maxEnglishWords = 30`, `maxTextLen = 1000`, `maxAuthorLen = 64`; `(*Source).Describe()`, `(*Source).Parse(ctx, r io.Reader, emit func(row any) error) error`. Строки отдаются в порядке файла; `Parse` по завершении пишет в `slog` одну строку со счётчиком пропущенных длинных пар. `Fetch` и `Load` в этой задаче — заглушки, возвращающие ошибку «не реализовано» (они нужны, чтобы тип удовлетворял `importer.Source`; заменяются в Task 3 и 4).
- `Describe`: лицензия и атрибуция — тексты из спека; URL — закреплённая ссылка.

- [ ] **Step 1: Write the failing tests.** `testdata/sample.txt` ≈ 40 строк, настоящий формат: первые строки с `Go.` (несколько русских переводов одного английского), русское предложение, общее для двух английских, строки с кавычками и кириллицей, автор с точкой и дефисом, одно английское предложение длиннее 30 слов; расчёт ожидаемых счётчиков (строк в файле, пропущенных, пар, уникальных английских и русских id) делается скриптом по образцу и вписывается в тест числами. `tatoeba_test.go`: `Describe` (имя `tatoeba`, версия `2026-02-13`, лицензия содержит «CC BY 2.0 FR», атрибуция называет Tatoeba и manythings.org, URL закреплён); разбор образца даёт ожидаемое число `Row` и верные поля первой, последней и «общих» строк (id, тексты, авторы); длинное пропущено; `\r\n` в конце строк не попадает в текст; ошибки с «строка N» и причиной: две колонки вместо трёх, четыре колонки, чужая лицензия в атрибуции (`CC0`), атрибуция без второго `#`, id не число, id равен нулю, пустое английское, пустое русское, пустой автор, недопустимый UTF-8 (`\xff`), нулевой байт, текст из 1001 символа, автор из 65 символов; пустой файл даёт ноль строк без ошибки; `emit`, вернувший ошибку, прерывает разбор; отменённый контекст прерывает разбор.
- [ ] **Step 2: Run** `go test ./internal/sources/tatoeba/...` — Expected: FAIL.
- [ ] **Step 3: Implement** по Interfaces (`bufio.Scanner` с буфером 1 МиБ, `regexp` с шаблоном атрибуции из Global Constraints, `utf8.ValidString`, `strconv.ParseInt`).
- [ ] **Step 4: Run** `go vet ./... && go test -race ./internal/sources/tatoeba/... && golangci-lint run` — Expected: PASS и чисто.
- [ ] **Step 5: Commit**: «Tatoeba: описание и разбор rus.txt».

### Task 3: `Fetch`: скачивание и безопасная распаковка

**Files:**
- Modify: `internal/sources/tatoeba/tatoeba.go`; Create: `internal/sources/tatoeba/fetch_test.go`

**Interfaces:**
- Consumes: `importer.Download`, `importer.NewHTTPClient`.
- Produces: непубличный `newSource(url, sha256 string, maxZip, maxExtract int64, client *http.Client) *Source` (для тестов); `(*Source).Fetch(ctx, cacheDir) (string, error)` кладёт zip в `<cacheDir>/sources/tatoeba/rus-eng.zip`, достаёт `rus.txt` в `<cacheDir>/sources/tatoeba/rus.txt` и возвращает этот путь; непубличный `extractMember(zipPath, member, dest string, maxBytes int64) error`.
- Правила распаковки: ищется запись с именем ровно `rus.txt` (без каталога); запись должна быть обычным файлом; читается не больше `maxBytes+1` байт независимо от заявленного в zip размера; запись идёт во временный файл рядом и переименовывается в конце; любая ошибка удаляет временный файл и не трогает уже лежащий `rus.txt`; `rus.txt` пересоздаётся при каждом `Fetch` (дёшево, исключает устаревший файл).

- [ ] **Step 1: Write the failing tests** (`httptest.NewTLSServer`, zip собирается в тесте через `archive/zip` в памяти): успех (файл `rus.txt` в кэше совпадает с исходным); повторный `Fetch` не качает zip заново (счётчик запросов = 1) и пересоздаёт `rus.txt`; zip без `rus.txt` → ошибка; zip, где есть только `../rus.txt`, `/rus.txt` или `dir/rus.txt` → ошибка и не создан ни один файл за пределами каталога кэша; `rus.txt` больше `maxExtract` → ошибка и нет остатков; zip, у которого заявленный размер записи меньше фактического (подделка заголовка, собирается вручную через `zip.Writer.CreateRaw` или правку заголовка) → ошибка на лимите чтения; каталог под именем `rus.txt` → ошибка; неверная сумма zip → ошибка до распаковки; в каталоге кэша после каждой неудачи нет `.download-*` и временных файлов; уже лежащий старый `rus.txt` не портится при неудачной распаковке.
- [ ] **Step 2: Run** `go test ./internal/sources/tatoeba/...` — Expected: FAIL.
- [ ] **Step 3: Implement** `Fetch` и `extractMember`; `New()` использует боевые константы (URL, сумма, 40 МиБ, 200 МиБ).
- [ ] **Step 4: Run** `go vet ./... && go test -race ./internal/sources/tatoeba/... && golangci-lint run` — Expected: PASS и чисто.
- [ ] **Step 5: Commit**: «Tatoeba: скачивание и безопасная распаковка».

### Task 4: `Load`, регистрация и сквозные тесты на БД

**Files:**
- Modify: `internal/sources/tatoeba/tatoeba.go`, `cmd/import/main.go` (одна строка регистрации + импорт); Create: `internal/sources/tatoeba/tatoeba_integration_test.go`

**Interfaces:**
- Consumes: `UpsertSentence`, `UpsertSentencePair` (Task 1), `Row` (Task 2), `importer.Run`, `testdb.Pool`.
- Produces: `(*Source).Load(ctx, q *db.Queries, sourceID int64, row any) error`: считает `word_count` через `strings.Fields`, вызывает `UpsertSentence` для английского (`lang = 'en'`) и русского (`'ru'`), сверяет возвращённые `Text` и `Author` с пришедшими (расхождение — ошибка с id и языком), затем `UpsertSentencePair`. Состояния между вызовами `Load` нет (никакого кэша: так повторный запуск после отката безопасен).

- [ ] **Step 1: Write the failing tests** (`tatoeba_integration_test.go`; TLS-сервер отдаёт zip с `rus.txt` из `testdata/sample.txt`, сумма считается из zip; путь к образцу вычисляется при старте пакета, потому что `testdb.Pool` меняет рабочий каталог): `importer.Run` — `sentences` и `sentence_pairs` содержат ожидаемые числа из образца, `data_sources.row_count` равен числу пар, атрибуция в записи источника содержит «CC BY 2.0 FR», у каждого предложения непустой автор и `tatoeba_id`, у предложения с несколькими переводами одна строка и несколько пар; двойной `Run` — те же числа, `imported_at` обновлён; тот же `*Source` после запуска, упавшего на середине (файл с битой строкой в конце), успешно импортирует корректный файл (нет залипшего состояния); битая строка в конце → ошибка с номером строки, в `data_sources`, `sentences`, `sentence_pairs` пусто; пустой файл и файл из одного слишком длинного английского предложения → ошибка «нет строк», `data_sources` пуст; конфликт — тот же `(en, id)` с другим текстом в двух строках → ошибка и откат; тот же `(ru, id)` с другим автором → ошибка; предложение, уже добавленное другим источником (вставка вручную через `Queries`), остаётся одной строкой с прежним `source_id`; неверная сумма zip → ошибка, пусто в базе. В `cmd/import/main_test.go` ничего не меняется (реестр там свой).
- [ ] **Step 2: Run** `go test ./internal/sources/tatoeba/...` — Expected: FAIL.
- [ ] **Step 3: Implement** `Load`; зарегистрировать `tatoeba.New()` в `cmd/import/main.go` одной строкой.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS и чисто; `go run ./cmd/import list` печатает `cefrj`, `octanove`, `tatoeba`.
- [ ] **Step 5: Commit**: «Источник Tatoeba».

### Task 5: Реальный импорт, замеры и документы

**Files:**
- Modify: `CLAUDE.md` (статус и команды — только проверенное); память проекта

- [ ] **Step 1: STOP и спросить пользователя:** можно ли применить миграцию `00003` к локальной базе `english_pass` (`set -a; . ./.env; set +a; goose -dir migrations postgres "$DATABASE_URL" up`; только создаёт две таблицы) и запустить реальный импорт `go run ./cmd/import tatoeba` (скачает 17 МБ с manythings.org, загрузит ≈ 806 тыс. предложений, займёт минуты). Без явного «да» ничего из этого не выполнять.
- [ ] **Step 2: После согласия:** применить миграцию; `time go run ./cmd/import tatoeba` — Expected: 536 106 пар за разумное время (если дольше ~20 минут — остановиться и предложить пакетную вставку отдельным шагом); проверить через `docker compose exec -T db psql`: `SELECT lang, count(*) FROM sentences GROUP BY lang` (en 341 165, ru 465 421), `SELECT count(*) FROM sentence_pairs` (536 106), `data_sources.row_count`, размеры таблиц `pg_total_relation_size`, выборка нескольких строк с авторами; повторный запуск даёт те же числа; `git status` без `.cache/`.
- [ ] **Step 3: Документы:** `CLAUDE.md` — абзац в «Статусе» (импорт Tatoeba сделан: 806 586 предложений и 536 106 пар, лицензия CC BY 2.0 FR, авторы сохранены; примеры слов и полнотекстовый индекс ещё нет) и упоминание `tatoeba` рядом с `cefrj`/`octanove` в «Командах» — после того, как команда реально отработала; в «Ещё нет» убрать Tatoeba.
- [ ] **Step 4: Финальная проверка:** `go vet ./... && go test -race ./... && golangci-lint run`; ревью (агент `reviewer`, один раз на подпроект).
- [ ] **Step 5: Commit** по просьбе пользователя: «Tatoeba: документы».
