# «Шпаргалки», часть 1: каркас и три первые шпаргалки

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (Native). Шаги с чекбоксами `- [ ]`.

**Goal:** раздел `/sheets`: поиск и теги, избранное, «умные» таблицы (статусы слов и тем пользователя, фильтры, печать), встроенные в сервер шпаргалки; три первые шпаргалки как `draft`.

**Architecture:** шпаргалки — JSON в `content/sheets/` (`go:embed`), валидатор схемы и загрузчик в чистом пакете `internal/sheets`; в базе только `sheet_state` (избранное, «проверено»). Сервер подмешивает статусы слов (`word_learning`, `cards`) и тем (`topic_ratings`) в строки с `bind`; «Мои ошибки» собирается из банка ошибок. Клиент рисует блоки, фильтрует и сортирует таблицы без запросов.

**Tech Stack:** Go, sqlc, goose, OpenAPI, React, Vitest, Playwright. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-26-cheatsheets-design.md`. Расхождение: номер миграции `00016` (`00015` занята «Знакомством»).

## Global Constraints
- Тексты пишутся заново, дословно из учебников ничего не копируем; у каждой шпаргалки `sources` не пуст и `status = "draft"`, пока пользователь не нажмёт «Проверено» (`reviewed_at` в `sheet_state`; статус в ответе API = `reviewed`, только если эта отметка есть).
- Миграции и данные: только `sheet_state`; сами шпаргалки в базе не хранятся.
- События `sheet.favorited`, `sheet.reviewed` (журнал `events`, ключ идемпотентности).
- Привязки: `word` (ключ — лемма), `topic` (ключ — тема), `mistake` (шпаргалка `mine-mistakes`, собирается сервером).
- Интерфейс на русском, учебный контент на английском; компоненты по `DESIGN.md`, новых токенов нет; `dangerouslySetInnerHTML` запрещён (тексты блоков — только текст, выделение формы через `**…**` разбирается своим разборщиком в `<strong>`).
- Миграция применяется к настоящей базе только после «да» пользователя и `pg_dump`.

## Review Focus
- Шпаргалка с битой ссылкой на лемму или повторным `id` не должна ломать сервер: загрузка падает на старте теста, не в рантайме.
- Пользователь без данных (нет карточек, слов, ошибок): таблица работает как обычная, «Мои ошибки» пустая с пояснением.
- Повтор `PUT favorite` и `PUT review` идемпотентен; чужой `sheet_id` → `404`.
- Длинные таблицы (50 глаголов) на 390 px: прокрутка по горизонтали, строки не ломают вёрстку; печать — чёрное на белом.
- Статус слова: `hard` при `lapses >= 3` приоритетнее `learning`; слово из лемм с вариантами через слэш.

---

### Task 1: Схема и загрузчик шпаргалок (чистые функции)
**Files:** Create `internal/sheets/sheet.go`, `sheet_test.go`, `content/sheets/embed.go` (`//go:embed *.json`).
**Interfaces:** Produces `type Sheet`, `type Block` (поле `type` ∈ `text|table|tip|mistakes|examples`), `func Parse(data []byte) (Sheet, error)`, `func LoadAll() ([]Sheet, error)` (проверяет уникальность `id`, непустые `sources`, уровни A1–C1, привязки `word|topic|mistake`, у `table` одинаковое число ячеек в строках, у `text` до 60 слов).
- [ ] Тесты: валидные и невалидные JSON по каждому правилу; `LoadAll` на встроенных файлах проходит.
- [ ] `go test ./internal/sheets` FAIL → реализовать → PASS. Commit «Шпаргалки: схема и загрузчик».

### Task 2: Первые три шпаргалки (draft)
**Files:** Create `content/sheets/to-be.json`, `irregular-verbs.json` (50 самых частых глаголов, строка = базовая форма, Past Simple, Past Participle, перевод; `bind: word`, `key` = лемма), `articles.json` (схема выбора, `bind: topic grammar:articles`); `content/sheets/mine-mistakes.json` (заглушка с `bind: mistake`, без строк).
- [ ] Тест содержимого: каждая шпаргалка проходит `LoadAll`; леммы `irregular-verbs` существуют в `lexemes` (интеграционный, во временной схеме или на настоящих данных; отсутствующие перечисляются в сообщении).
- [ ] Тексты по формату спека (пример `to-be` — оттуда), всё `status: draft`, `sources` честные («собственные знания автора; сверить с CEFR-J Grammar Profile»). Commit «Шпаргалки: первые три».

### Task 3: Состояние и подмешивание данных пользователя
**Files:** Create `migrations/00016_sheet_state.sql` (`sheet_state(user_id, sheet_id text, favorite_at, reviewed_at, opened_at, PRIMARY KEY (user_id, sheet_id))`; откат `DROP TABLE`), `queries/sheets.sql` (`SheetStates`, `SheetSetFavorite`, `SheetClearFavorite`, `SheetSetReviewed`, `SheetTouchOpened`, `SheetWordStatuses :many` по леммам: статус из `word_learning`, `lapses` из `cards`, `SheetOpenMistakes`), `internal/sheets/status.go` + тест.
**Interfaces:** `func WordStatus(learning *string, hasCard bool, lapses int) string` → `unseen|introduced|learning|learned|known_at_start|hard`; `func TopicWeak(rating float64, all []float64) bool` (нижняя треть); `func MineMistakes(rows []MistakeRow) Sheet`.
- [ ] Тесты чистых функций (границы `lapses` 2/3, нижняя треть при 3 темах, пустой банк ошибок).
- [ ] Интеграционный тест запросов на Postgres во временной схеме; `sqlc generate`. Commit «Шпаргалки: состояние и статусы».

### Task 4: Эндпоинты
**Files:** Modify `api/openapi.yaml` (`GET /sheets`, `GET /sheets/{id}`, `PUT/DELETE /sheets/{id}/favorite`, `PUT /sheets/{id}/review`; схемы `SheetSummary`, `Sheet`, `SheetBlock`, `Table`, коды ошибок не добавляются, `404 not_found`), `internal/httpapi/sheets.go`, `sheets_test.go`, `auth.go` + фейк.
- [ ] Тесты (фейк): список с `favorite`, `status`, числом «моих слабых»; детали подмешивают статусы слов и тем; пустой пользователь; `mine-mistakes` из банка ошибок; повторные `PUT` идемпотентны, одно событие; чужой id → 404; `DELETE` без избранного → 204.
- [ ] `go generate ./internal/api`, `npm run gen:api`, `go vet ./... && go test -race ./...`. Commit «Шпаргалки: эндпоинты».
- Миграция на настоящую базу — только после «да» пользователя.

### Task 5: Экраны
**Files:** Create `web/src/features/sheets/` (`SheetsPage.tsx` список, `SheetPage.tsx`, `SmartTable.tsx`, `richText.tsx` разбор `**…**`, `sheets.ts` запросы, тесты), `web/e2e/sheets.spec.ts`; Modify `router.tsx`, `AppShell` (пункт «Шпаргалки», иконка), `TodayPage` (блок «Закреплённое», до трёх), `mock-api.ts`, `index.css` (стиль печати), `DESIGN.md`.
- Skills: до компонентов `impeccable`, `emil-design-eng`; после — `web-design-guidelines`, `break-ui`.
- [ ] Vitest: список, поиск, теги, избранное, плашка «Черновик»; таблица: статусы значком и словом, фильтры «Все / Мои слабые / Ещё не учил», сортировка и поиск по столбцу; разбор `**…**` не создаёт HTML; пустые данные.
- [ ] `npx vitest run && npx tsc -b && npm run lint && npm run e2e` (мышь, клавиатура, обе темы, 1440/390, печать через `emulateMedia({media: 'print'})`). Commit «Шпаргалки: экраны».

### Task 6: Живая проверка, документация, ревью
- [ ] Песочница: `goose up` до 00016 (только песочница), сервер на 8082, временный пароль (файл удалить): список, подмешивание статусов (слова из `word_learning`), «Мои ошибки» после неверного ответа, избранное, `review`.
- [ ] `CLAUDE.md` (что есть, что НЕ проверено, порядок внедрения: `pg_dump` → `goose up` → сервер), `reviewer` один раз, исправления, commit «Шпаргалки: проверка и документация».

### Следующие части (отдельные планы после проверки содержимого пользователем)
Остальные 11 шпаргалок авторского пакета 1 пачками по 3–4: каждая пачка показывается пользователю («показал — подтвердил или вернул»), только потом `reviewed`.

## Self-review
Спек покрыт: формат и валидатор (T1), содержимое-образец и политика (T2), статусы и умность (T3–T5), избранное и «Закреплённое» (T4–T5), печать (T5), «Мои ошибки» (T3–T4), проверка человеком (следующие части). Не входит и отложено: ссылки «Шпаргалка по теме» в «Привале» и «Ошибках», «Добавить в Знакомство» из строки (нужен приоритет слова в блоке `/learn/block` — отдельное изменение), остальные 11 шпаргалок.
