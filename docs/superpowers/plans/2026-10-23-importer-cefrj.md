# Каркас импортёра и источник CEFR-J — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `go run ./cmd/import cefrj` загружает слова CEFR-J (уровни A1–B2) в PostgreSQL; повторный запуск ничего не дублирует; новый источник подключается модулем без правок каркаса.

**Architecture:** Пакет `internal/importer` задаёт контракт `Source`, реестр, безопасное скачивание и запуск `Run` (Fetch вне транзакции, Parse + Load построчно в одной транзакции). Модуль `internal/sources/cefrj` реализует контракт. Три новые таблицы (`data_sources`, `lexemes`, `lexeme_cefr`) приходят одной миграцией; SQL только в `queries/` через sqlc.

**Tech Stack:** Go 1.27 (`encoding/csv`, `net/http`, `crypto/sha256`), pgx v5, sqlc 1.31.1, goose 3.29. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-23-importer-cefrj-design.md`

## Global Constraints

- Не добавлять зависимости в `go.mod`; всё из стандартной библиотеки и уже подключённых pgx и sqlc.
- SQL только в `queries/`, без сборки строк. Код sqlc перегенерируется командой `sqlc generate` (в PATH), `internal/db/*.go` руками не правится.
- Миграцию `00001_init.sql` не менять; новая — `migrations/00002_knowledge_base.sql`, с рабочим down. Через навык `db-migration`. Применять на реальную локальную базу только с явного согласия пользователя (см. Task 5).
- Скачанные файлы лежат в `.cache/sources/cefrj/` (уже в `.gitignore`) и в Git не попадают; недоверенные данные.
- Источник закреплён: URL `https://raw.githubusercontent.com/openlanguageprofiles/olp-en-cefrj/d4e45b75b38f27b30dfc5c44d8c571aec7e7092f/cefrj-vocabulary-profile-1.5.csv`, SHA-256 `b0dd3c635f1c9a4fdf1490c7e5b7c48e8bbe55b652ad0c9860a95f98e10ae498`, размер 233 214 байт, 7 799 строк (A1 1 164, A2 1 411, B1 2 446, B2 2 778).
- Атрибуция дословно: `The CEFR-J Wordlist Version 1.5. Compiled by Yukio Tono, Tokyo University of Foreign Studies.` Имя источника `cefrj`, версия `1.5`.
- Скачивание: только HTTPS (редирект на не-HTTPS отклоняется), таймаут, лимит 10 МиБ, имя файла задаёт модуль.
- Комментарии и сообщения об ошибках по-русски, в стиле соседнего кода. Коммиты по просьбе пользователя; перед коммитом `go vet ./... && go test -race ./... && golangci-lint run`.

## Review Focus

- Файл без данных (только заголовок или пустой): ошибка и откат, `data_sources` без строки с нулём (Task 3, 4).
- Ошибка на середине файла: откат всего, ни одного слова из этого запуска в `lexemes` (Task 3, 4).
- Файл с BOM в начале и поля с запятыми в кавычках (`"News, lifestyles and current affairs"`) разбираются верно (Task 4).
- Одна пара (слово, часть речи) от двух источников: второй не меняет `lexemes.source_id`, получает свою строку `lexeme_cefr` (Task 3).
- Сбой скачивания (несовпадение суммы, не 200, обрыв, превышение лимита, редирект на HTTP) не оставляет файла в кэше и не портит уже хороший файл (Task 2).
- Повторный импорт той же версии: те же числа строк, `row_count` и `imported_at` обновлены (Task 3, 4).

---

### Task 1: Миграция, запросы sqlc и общий помощник тестовой базы

**Files:**
- Create: `migrations/00002_knowledge_base.sql`, `queries/sources.sql`, `internal/testdb/testdb.go`, `internal/testdb/testdb_test.go`
- Modify: `internal/httpapi/integration_test.go` (заменить `integrationPool`/`moduleRoot` вызовами `testdb.Pool`)
- Regenerate: `internal/db/*.go` (`sqlc generate`)

**Interfaces:**
- Produces (SQL → sqlc): методы `*db.Queries`: `UpsertDataSource(ctx, db.UpsertDataSourceParams{Name, Version, License, Url, Attribution string}) (db.DataSource, error)`, `FinishDataSource(ctx, db.FinishDataSourceParams{ID int64, RowCount int32}) error`, `GetDataSource(ctx, db.GetDataSourceParams{Name, Version string}) (db.DataSource, error)`, `UpsertLexeme(ctx, db.UpsertLexemeParams{Lemma, Pos string, SourceID int64}) (int64, error)` (возвращает id; уже существующее слово не меняется), `UpsertLexemeCefr(ctx, db.UpsertLexemeCefrParams{LexemeID, SourceID int64, Level string, Extra json.RawMessage}) error`. Точные имена полей сверить по сгенерированному коду.
- Produces (Go): `testdb.Pool(t *testing.T) *pgxpool.Pool` — временная схема в базе из `DATABASE_URL`, все миграции применены по порядку (`Up`-части), схема удаляется после теста, нет базы — `t.Skip`; `testdb.Up(t, pool)` и `testdb.Down(t, pool)` — применить все `Up` по порядку / все `Down` в обратном порядке.

DDL (точные значения из спека): `data_sources(id bigint identity PK, name text NOT NULL CHECK length 1–32, version text NOT NULL CHECK length 1–32, license text NOT NULL, url text NOT NULL, attribution text NOT NULL, row_count integer CHECK >= 0, imported_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (name, version))`; `lexemes(id bigint identity PK, lemma text NOT NULL CHECK length 1–64, pos text NOT NULL CHECK length 1–32, source_id bigint NOT NULL REFERENCES data_sources ON DELETE RESTRICT, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (lemma, pos))`; `lexeme_cefr(lexeme_id bigint NOT NULL REFERENCES lexemes ON DELETE CASCADE, source_id bigint NOT NULL REFERENCES data_sources ON DELETE RESTRICT, level text NOT NULL CHECK IN ('A1','A2','B1','B2','C1','C2'), extra jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(extra) = 'object'), PRIMARY KEY (lexeme_id, source_id))`; индекс `lexemes (source_id)` и `lexeme_cefr (source_id)` под удаление и под подсчёт строк источника. Down: `DROP TABLE lexeme_cefr; DROP TABLE lexemes; DROP TABLE data_sources;`.

- [ ] **Step 1: Write the failing tests** в `testdb_test.go`:
  - `TestMigrationsUpDownUp`: `Pool(t)` уже применил `Up`; затем `Down(t, pool)` и таблиц `users`, `data_sources`, `lexemes`, `lexeme_cefr` в схеме нет (`to_regclass(...) IS NULL`); затем `Up(t, pool)` снова — все четыре таблицы есть.
  - `TestKnowledgeSchemaConstraints`: вставка в `lexeme_cefr` с уровнем `'D1'`, `extra = '[]'`, повтор `(lemma, pos)` в `lexemes`, `data_sources` с одинаковой парой `(name, version)` — каждая даёт ошибку PostgreSQL (23514 / 23505).
  - Тесты `internal/httpapi` продолжают проходить после замены помощника (отдельного теста не нужно).
- [ ] **Step 2: Run** `go test ./internal/testdb/...` — Expected: FAIL (пакета нет).
- [ ] **Step 3: Implement** `internal/testdb/testdb.go`: логика из `integrationPool` (`t.Chdir(moduleRoot)`, `config.Load`, схема `test_<наносекунды>`, `search_path`, cleanup), но миграции читаются из `migrations/*.sql` по имени по возрастанию, у каждой берутся части между `-- +goose Up` и `-- +goose Down` (Up) и после `-- +goose Down` (Down); директивы `-- +goose StatementBegin/End` из текста вырезаются, как и сейчас в `00001`. Создать `migrations/00002_knowledge_base.sql` и `queries/sources.sql` по Interfaces; `sqlc generate`.
- [ ] **Step 4: Replace** в `internal/httpapi/integration_test.go` `integrationPool(t)` на `testdb.Pool(t)` и удалить `integrationPool` и `moduleRoot`.
- [ ] **Step 5: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: всё PASS, в том числе `TestAuthFlowOnRealDatabase` и `TestSettingsFlowOnRealDatabase` (база поднята, не SKIP: проверить по `-v`).
- [ ] **Step 6: Commit**: `migrations/00002_knowledge_base.sql queries/sources.sql internal/db internal/testdb internal/httpapi/integration_test.go`, «Схема знаний: data_sources, lexemes, lexeme_cefr».

### Task 2: Безопасное скачивание с проверкой суммы

**Files:**
- Create: `internal/importer/download.go`, `internal/importer/download_test.go`

**Interfaces:**
- Produces: `importer.Download(ctx context.Context, client *http.Client, url, dest, wantSHA256 string, maxBytes int64) error` — если `dest` уже есть и его SHA-256 верна, сеть не трогается; иначе качает во временный файл рядом с `dest`, проверяет размер и сумму, и только потом атомарно переименовывает. Ошибка оставляет `dest` как был (хороший файл не портится, плохого не создаётся), временный файл удаляется.
- Produces: `importer.NewHTTPClient() *http.Client` — клиент с таймаутом и `CheckRedirect`, отвергающим редирект не на `https`.

- [ ] **Step 1: Write the failing tests** (`httptest.NewTLSServer`, клиент `srv.Client()`, но с тем же `CheckRedirect`): успех (файл создан, содержимое совпало); повторный вызов при верном файле не делает запроса (счётчик на сервере = 1); неверная сумма → ошибка, `dest` не существует; ответ 404/500 → ошибка; тело больше `maxBytes` → ошибка и `dest` не создан; обрыв соединения посреди тела → ошибка, временных файлов в каталоге нет; URL со схемой `http://` → ошибка до запроса; редирект на `http://…` → ошибка; уже лежащий *повреждённый* `dest` заменяется хорошим файлом, а при неудачной загрузке остаётся нетронутым.
- [ ] **Step 2: Run** `go test ./internal/importer/...` — Expected: FAIL.
- [ ] **Step 3: Implement** `Download` и `NewHTTPClient` по Interfaces; чтение тела через `io.LimitReader(maxBytes+1)` и сверка суммы на лету (`io.TeeReader` в `sha256`).
- [ ] **Step 4: Run** `go test -race ./internal/importer/...` — Expected: PASS.
- [ ] **Step 5: Commit**: «Импортёр: скачивание с проверкой суммы».

### Task 3: Контракт, реестр и запуск `Run`

**Files:**
- Create: `internal/importer/importer.go` (контракт, реестр), `internal/importer/run.go`, `internal/importer/run_test.go`, `internal/importer/registry_test.go`

**Interfaces:**
- Consumes: `db.Queries` методы из Task 1; `testdb.Pool`.
- Produces:
  - `type Info struct{ Name, Version, License, URL, Attribution string }`
  - `type Source interface { Describe() Info; Fetch(ctx context.Context, cacheDir string) (path string, err error); Parse(ctx context.Context, r io.Reader, emit func(row any) error) error; Load(ctx context.Context, q *db.Queries, sourceID int64, row any) error }` — `Parse` строки не накапливает, `Load` вызывается раннером на каждую эмитированную строку внутри транзакции.
  - `type Registry`; `NewRegistry() *Registry`; `(*Registry).Register(s Source)` (паника на повторное имя); `(*Registry).Get(name string) (Source, bool)`; `(*Registry).Names() []string` (по алфавиту).
  - `type Beginner interface{ Begin(ctx context.Context) (pgx.Tx, error) }` (его реализует `*pgxpool.Pool`); `type Result struct{ Info Info; Rows int; Duration time.Duration }`; `Run(ctx context.Context, pool Beginner, src Source, cacheDir string) (Result, error)`.
- Поведение `Run`: `Fetch` → открыть файл → транзакция → `UpsertDataSource` → `Parse` с `emit`, вызывающим `Load` и считающим строки → если строк 0, ошибка «в источнике нет строк» → `FinishDataSource(id, rows)` → commit. Любая ошибка (в том числе из `Load`, `Parse`, отмена контекста) откатывает всё. Ошибки оборачиваются с именем источника.

- [ ] **Step 1: Write the failing tests** (фейковый `Source` в `run_test.go`, пишет строки `fakeRow{Lemma, Pos, Level string}` в `lexemes` и `lexeme_cefr` через `Queries`, база из `testdb.Pool`): успешный запуск → в `lexemes` и `lexeme_cefr` столько строк, сколько эмитировано, `data_sources.row_count` и `imported_at` заполнены, `Result.Rows` верен; двойной запуск той же версии → те же числа строк, `imported_at` обновился; фейк, падающий на 3-й строке → ошибка, в `lexemes`, `lexeme_cefr` и `data_sources` ничего (откат); фейк без строк → ошибка и пустой `data_sources`; два фейка с разными именами и одной парой `(lemma, pos)` → один ряд в `lexemes` с `source_id` первого, два ряда в `lexeme_cefr`; `Fetch` возвращает ошибку → транзакция не открывалась (в `data_sources` пусто); отменённый контекст → ошибка. `registry_test.go`: `Register` + `Get`, `Names` по алфавиту, повторное имя вызывает панику, `Get` неизвестного даёт `false`.
- [ ] **Step 2: Run** `go test ./internal/importer/...` — Expected: FAIL.
- [ ] **Step 3: Implement** по Interfaces.
- [ ] **Step 4: Run** `go vet ./... && go test -race ./internal/importer/... && golangci-lint run` — Expected: PASS и чисто.
- [ ] **Step 5: Commit**: «Импортёр: контракт, реестр и запуск».

### Task 4: Модуль CEFR-J

**Files:**
- Create: `internal/sources/cefrj/cefrj.go`, `internal/sources/cefrj/cefrj_test.go`, `internal/sources/cefrj/cefrj_integration_test.go`, `internal/sources/cefrj/testdata/sample.csv`

**Interfaces:**
- Consumes: `importer.Source`, `importer.Info`, `importer.Download`, `importer.NewHTTPClient`, `db.Queries`.
- Produces: `cefrj.New() *Source` (боевой URL, сумма и размер из Global Constraints; `Name()` = `cefrj`); непубличный конструктор `newSource(url, sha256 string, maxBytes int64, client *http.Client) *Source` для тестов; тип строки `cefrj.Row{Headword, POS, Level string; Extra map[string]string}` (только непустые `CoreInventory 1`, `CoreInventory 2`, `Threshold` под ключами `coreInventory1`, `coreInventory2`, `threshold`).
- `Fetch` кладёт файл в `<cacheDir>/sources/cefrj/cefrj-vocabulary-profile-1.5.csv`. `Parse` читает CSV потоком, убирает BOM, проверяет заголовок (ровно эти шесть колонок), требует непустые `headword`/`pos`/`CEFR`, уровень из A1–C2, `headword` до 64 и `pos` до 32 символов; ошибка называет номер строки. `Load` — `UpsertLexeme` и `UpsertLexemeCefr` (`Extra` в JSON).

- [ ] **Step 1: Write the failing tests.** `testdata/sample.csv` ≈ 30 строк с настоящими заголовками: строки с запятой в кавычках (`"News, lifestyles and current affairs"`), варианты со слэшем (`a.m./A.M./am/AM`, `cafe/café`), `modal auxiliary`, `infinitive-to`, пустые необязательные поля, все четыре уровня. `cefrj_test.go`: `Describe()` отдаёт имя `cefrj`, версию `1.5`, дословную атрибуцию, URL закреплён на коммит `d4e45b75…`; `Parse` образца даёт нужное число строк и поля `Extra` (в том числе запятую внутри значения); BOM в начале не мешает; ошибки с номером строки — другой заголовок, пустой `headword`, уровень `D1`, `headword` из 65 символов, файл без данных (только заголовок) и пустой файл; `emit`, вернувший ошибку, прерывает разбор. `cefrj_integration_test.go` (база из `testdb.Pool`, TLS-сервер отдаёт `sample.csv`, сумма считается из файла): `importer.Run` — в `lexeme_cefr` столько строк, сколько в образце, `lexemes.source_id` равен id источника, `extra` у строки с `CoreInventory` содержит нужный ключ, `data_sources` содержит атрибуцию; второй `Run` — те же числа; файл с битой строкой в конце → ничего не записано; сумма не сходится → ошибка Fetch и пустая база.
- [ ] **Step 2: Run** `go test ./internal/sources/...` — Expected: FAIL.
- [ ] **Step 3: Implement** модуль по Interfaces (`encoding/csv` с `FieldsPerRecord = 6`, `ReuseRecord`; номер строки из `Reader.FieldPos`).
- [ ] **Step 4: Run** `go vet ./... && go test -race ./... && golangci-lint run` — Expected: PASS и чисто.
- [ ] **Step 5: Commit**: «Источник CEFR-J».

### Task 5: Команда `cmd/import`, реальный импорт и документы

**Files:**
- Create: `cmd/import/main.go`, `cmd/import/main_test.go`
- Modify: `CLAUDE.md` (статус и команды — только проверенные), память проекта

**Interfaces:**
- Consumes: `importer.NewRegistry`, `importer.Run`, `cefrj.New`, `config.Load`.
- Produces: `go run ./cmd/import <имя>` и `go run ./cmd/import list`. Логика вынесена в `run(ctx, args []string, stdout, stderr io.Writer, reg *importer.Registry, open func(ctx) (importer.Beginner, func(), error)) error`, чтобы тестироваться без базы; регистрация источника в `main` одной строкой `reg.Register(cefrj.New())`.

- [ ] **Step 1: Write the failing tests** (`main_test.go`, фейковые реестр и «база»): `list` печатает имена по алфавиту; запуск неизвестного имени → ошибка с перечнем доступных; без аргументов и с двумя аргументами → ошибка с подсказкой использования; успешный запуск печатает имя, версию, число строк и время; ошибка источника возвращается наружу (код выхода не нулевой — проверяется на уровне `main`).
- [ ] **Step 2: Run** `go test ./cmd/import/...` — Expected: FAIL.
- [ ] **Step 3: Implement** `cmd/import/main.go` по образцу `cmd/createuser`: `config.Load`, пул `pgxpool`, `slog`, при ошибке `os.Exit(1)`; кэш в `.cache`.
- [ ] **Step 4: Verify by running**: `go run ./cmd/import list` печатает `cefrj`; `go run ./cmd/import nope` завершается с ошибкой. Затем **остановиться и спросить пользователя**: можно ли применить миграцию `00002` к локальной базе `english_pass` (`set -a; . ./.env; set +a; goose -dir migrations postgres "$DATABASE_URL" up`; миграция только добавляет таблицы, данные не трогает) и запустить реальный импорт (скачает 233 КБ с raw.githubusercontent.com). Без явного «да» эти два действия не выполнять.
- [ ] **Step 5: После согласия:** применить миграцию (цикл up → down → up на реальной базе не гонять: он уже проверен в `TestMigrationsUpDownUp` на временной схеме); `go run ./cmd/import cefrj` — Expected: 7 799 строк; затем `psql`/`docker compose exec` или Go-запросом проверить `SELECT level, count(*) FROM lexeme_cefr GROUP BY level` — A1 1 164, A2 1 411, B1 2 446, B2 2 778; повторный запуск — те же числа и обновлённый `imported_at`; `git status` не показывает `.cache/`.
- [ ] **Step 6: Документы:** `CLAUDE.md` — абзац в «Статусе» («каркас импортёра и источник CEFR-J сделаны; страницы лицензий, поиска, Tatoeba, частот, Octanove C1/C2 ещё нет») и две строки в «Команды» (`go run ./cmd/import list`, `go run ./cmd/import cefrj`, `goose … up` уже описан) — только после того, как они реально отработали.
- [ ] **Step 7: Финальная проверка:** `go vet ./... && go test -race ./... && golangci-lint run`; ревью (агент `reviewer`, один раз на подпроект).
- [ ] **Step 8: Commit** по просьбе пользователя: «Команда import и документы».
