# «Знакомство и темп» (этап A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Пользователь знакомится со словами до того, как они попадают в карточки: блок слов с переводом, IPA, звуком и примерами из базы знаний, быстрый тест «знаю / не знаю», адаптивный дневной лимит новых слов, состояние «изучено» по первому хорошему вспоминанию.

**Architecture:** Таблицы `word_learning` и `learn_preferences` (миграция `00015`). Чистые функции `internal/learn` (подбор блока) и `internal/pace` (лимит новых слов). Эндпоинты `/learn/*` читают `kb_teaching_card`/`kb_lexeme_stat` и существующие примеры Tatoeba; `ReviewCard` переводит слово в `learned`. Фронтенд: страницы `/learn` и `/learn/placement`, карточка на «Сегодня», блок «Обучение» в настройках. «Привал» — отдельный этап B.

**Tech Stack:** Go, pgx + sqlc, goose, oapi-codegen, React + TypeScript, TanStack Query, Vitest, Playwright. Новых зависимостей нет.

**Spec:** `docs/superpowers/specs/2026-10-26-learn-camp-design.md` (раздел «Уточнения» главнее остального текста), база знаний — `2026-10-26-knowledge-base-design.md`.

## Global Constraints

- Без новых зависимостей. Миграцию `00015` проверять во временной схеме; к настоящей базе **не применять без `pg_dump` и «да» пользователя**.
- SQL только в `queries/` через sqlc; контракт меняется сначала в `api/openapi.yaml`, затем `go generate ./internal/api` и `npm run gen:api`; генерированное руками не править.
- Чистая логика (`PickBlock`, `NewWordsToday`) — функции без базы с табличными юнит-тестами.
- В блок попадают только слова с выбранным русским переводом в `kb_teaching_card` и хотя бы одним примером Tatoeba (`exampleSentences`); блок не больше 7 слов; не больше 3 слов одной части речи; слова-синонимы и слова, отличающиеся одной буквой, в один блок не ставятся.
- Лимит новых слов в день: авто-режим (формула в спеке: база `clamp(round(минуты/3), 3, потолок)`, просрочено больше `7 × база` — 0, точность 7 суток < 0,70 — база − 2, ≥ 0,90 и просрочено ≤ база — база + 2) или ручной (`new_words_per_day`, 1–30); сутки по часовому поясу настроек; потолок авто-режима по умолчанию 12; покрытие «Привала» 0,90–0,99 (по умолчанию 0,95).
- Слово становится `learned` только в транзакции `ReviewCard` при оценке «хорошо» (3) или «легко» (4); нет строки `word_learning` — создаётся `learned` с `source = 'card'`.
- Тексты из базы выводятся только текстом; интерфейс по-русски, по `DESIGN.md`; движение только для мыши и без «уменьшить движение».
- Проверка перед каждым коммитом: `go vet ./... && go test -race ./... && golangci-lint run`, в `web/` — `npx vitest run && npx tsc -b && npm run lint`.

## Review Focus

- Слово уже в карточках или `word_learning` (любой статус): в блок не попадает; повторное `introduce` по тому же ключу ничего не дублирует.
- Нет `kb` (пустые таблицы) или все слова изучены: `404 no_words` с русским текстом, не 500.
- Лимит дня исчерпан: `introduce` возвращает 409; в авто-режиме просроченных больше порога — лимит 0 с причиной.
- «Хорошо» на карточке слова с `introduced` даёт `learned` один раз; «забыл» и «трудно» не дают; повтор запроса по ключу не пишет второй `word.learned`.
- Быстрый тест: «знаю» не создаёт карточек; повтор по ключу не дублирует; чужие и несуществующие номера слов отклоняются.
- Часовой пояс: слово, введённое в 23:50 и в 00:10 по поясу пользователя, считается в разные сутки.

---

### Task 1: Таблицы `word_learning`, `learn_preferences` и запросы

**Files:**
- Create: `migrations/00015_word_learning.sql`, `queries/learn.sql`, `internal/testdb/learn_test.go`
- Regenerate: `internal/db` (`sqlc generate`)

**Interfaces:**
- Produces: таблицы `word_learning (user_id, lexeme_id, status, source, introduced_at, learned_at; PK user_id+lexeme_id)` и `learn_preferences (user_id PK, new_words_mode, new_words_per_day, camp_coverage, updated_at)`; перенос: карточки со `state = 2` → `learned`, `source = 'legacy_card'`. Запросы sqlc:
  - `LearnCandidates(user_id, levels text[], max_rows)` — слова из `kb_teaching_card` с переводом (`jsonb_array_length(payload->'translations') > 0`), уровнем из `lexeme_cefr` (самый низкий), без строки `word_learning` и без карточки пользователя, по уровню затем `kb_lexeme_stat.freq_rank` (NULL последним) и `id`; поля `lexeme_id, lemma, pos, level, freq_rank, payload`;
  - `LearnHardWords(user_id, max_rows)` — карточки с `lapses >= 3` (слово, `payload`), без `status = 'known_at_start'`;
  - `LearnRelations(lexeme_ids)` — `lexeme_id, other_lexeme_id` из `kb_relation` (только с `other_lexeme_id IS NOT NULL`);
  - `LearnStatusCounts(user_id)` — число слов по `status`;
  - `LearnIntroducedSince(user_id, since)` — число строк `word_learning` с `introduced_at >= since` и `source = 'lesson'`;
  - `LearnInsertIntroduced(user_id, lexeme_id, now)` (`ON CONFLICT DO NOTHING`, возвращает, вставлена ли строка), `LearnMarkKnown(user_id, lexeme_id, now)` (`known_at_start`, `ON CONFLICT DO NOTHING`), `LearnMarkLearned(user_id, lexeme_id, now)` (`INSERT … 'learned','card' ON CONFLICT DO UPDATE SET status='learned', learned_at=now WHERE word_learning.status='introduced'`, возвращает, изменилась ли строка);
  - `LearnReviewStats(user_id, since)` — число и число верных (`rating >= 3`) `card.reviewed` в `events` с `since`;
  - `LearnGetPreferences(user_id)`, `LearnUpsertPreferences(...)`;
  - `LearnPlacementCandidates(user_id, levels, max_rows, offset)` — как `LearnCandidates`, но без требования `payload` (достаточно слова из `lexemes` с `kb_lexeme_stat`), страницы по 50.

- [ ] **Step 1: Тест (красный)** `TestLearnTables`: после `Pool(t)` есть обе таблицы; статус вне списка и `learned` без `learned_at` отклоняются; перенос: карточка пользователя со `state = 2` до миграции даёт `learned/legacy_card` (проверить SQL переноса, выполнив его в тесте на подготовленных строках); `LearnMarkLearned` переводит `introduced` в `learned` и возвращает `true`, повторно — `false`; создаёт `learned/card` при отсутствии строки; `LearnInsertIntroduced` повторно ничего не вставляет; `LearnCandidates` не отдаёт слова с карточкой или строкой `word_learning` и слова без перевода в `kb_teaching_card`, отдаёт по рангу частоты; `LearnReviewStats` считает события `card.reviewed` с рейтингом; откат миграции убирает обе таблицы и не трогает `cards`.
- [ ] **Step 2:** `go test ./internal/testdb -run Learn` → FAIL.
- [ ] **Step 3:** Написать миграцию (Down: `DROP TABLE learn_preferences; DROP TABLE word_learning;`) и `queries/learn.sql`; `sqlc generate`.
- [ ] **Step 4:** `go test ./internal/testdb/...` → PASS.
- [ ] **Step 5: Commit** `git commit -m "Знакомство: таблицы word_learning и learn_preferences, запросы"`.

---

### Task 2: Чистая логика (`internal/learn`, `internal/pace`)

**Files:**
- Create: `internal/learn/block.go`, `internal/learn/block_test.go`, `internal/pace/pace.go`, `internal/pace/pace_test.go`

**Interfaces:**
- Produces:
  - `learn.Candidate{ID int64; Lemma, Pos string; Revisit bool}`;
  - `learn.PickBlock(cands []Candidate, n int, ok func(Candidate) bool, related func(a, b int64) bool) []Candidate` — идёт по списку по порядку (повторное знакомство уже поставлено первым), берёт слово, если `ok` (есть пример) и не больше трёх одной части речи, нет связи `related` ни с одним взятым, нет пары в одну букву (`oneLetterApart(lemma1, lemma2)`) с взятым; не больше `n`; `n <= 0` — пусто;
  - `pace.Input{DailyMinutes, Due int; Accuracy7d float64; HasAccuracy bool; IntroducedToday int; Mode string; PerDay, Cap int}`, `pace.Result{Limit, Remaining int; Reason Reason}`, `pace.Reason{Code, Text string}`, `pace.NewWordsToday(in Input) Result`; коды причин `manual`, `backlog`, `hard`, `easy`, `base`, `done`.

- [ ] **Step 1: Тесты (красные).** `TestPickBlock`: порядок сохраняется; лимит `n`; не больше трёх одной части речи; синоним (`related` в обе стороны) пропускается; `quite/quiet` вместе нельзя; слово без примера (`ok` ложно) пропускается; `n = 0` и пустой список; повторное знакомство остаётся первым. `TestNewWordsToday` (таблица): 15 минут без данных → база 5; 60 минут → 12 при потолке 12 (`round(60/3)=20` режется потолком); 5 минут → 3 (нижняя граница); просрочено > `7×база` → 0 `backlog`; точность 0,6 → база − 2 `hard` (не меньше 1); точность 0,92 и просрочено ≤ база → база + 2 `easy` (не выше потолка); точность без данных (`HasAccuracy` ложно) → 0,8 → база; из результата вычитается `IntroducedToday`, `Remaining` не отрицателен, при `Remaining = 0` причина `done`; ручной режим: `Limit = PerDay`, причина `manual`; `PerDay` вне 1–30 приводится к границам; детерминированность.
- [ ] **Step 2:** `go test ./internal/learn ./internal/pace` → FAIL.
- [ ] **Step 3:** Реализовать функции; тексты причин по-русски («Сначала повторим накопившееся», «Последние повторения шли трудно», «Повторения идут легко», «Ты выбрал число вручную», «На сегодня всё»).
- [ ] **Step 4:** `go test -race ./internal/learn ./internal/pace` → PASS.
- [ ] **Step 5: Commit** `git commit -m "Знакомство: подбор блока и дневной лимит новых слов (чистые функции)"`.

---

### Task 3: Контракт и обработчики сервера

**Files:**
- Modify: `api/openapi.yaml` (пути `/learn/status`, `/learn/block`, `/learn/introduce`, `/learn/placement` (GET, POST), `/learn/preferences` (GET, PUT); схемы `LearnStatus`, `LearnWord`, `LearnBlock`, `IntroduceRequest/Response`, `PlacementPage`, `PlacementRequest/Response`, `LearnPreferences`, код ошибки `no_words` и `limit_reached`), `internal/httpapi/auth.go` (методы хранилища), `internal/httpapi/cards.go` (`ReviewCard`: перевод слова в `learned`)
- Create: `internal/httpapi/learn.go`, `internal/httpapi/learn_test.go`, `internal/httpapi/learn_fake_test.go`

**Interfaces:**
- Consumes: запросы Task 1, `learn.PickBlock`, `pace.NewWordsToday`, `exampleSentences` (`cards.go`), `srs.New`, `insertParams`, `activity.Location` (часовой пояс настроек; найти по `grep -n "func Location" internal/activity/*.go`), `GetSettings`.
- Produces: поведение эндпоинтов по спеку: `GET /learn/status` (`learned`, `introduced`, `knownAtStart`, `newToday`, `limit`, `remaining`, `reason{code,text}`, `mode`, `perDay`, `kbReady`); `GET /learn/block` (`words: LearnWord[]` с `lexemeId, lemma, pos, level, rank?, translations[], ipa?, audioUrl?, forms[], synonyms[], examples[{en, ru, authorEn, tatoebaId, highlight?}], revisit`; `404 no_words`); `POST /learn/introduce {lexemeIds (1–7), key}` → `{introduced, cardIds}` (карточки и строки `introduced`, событие `word.introduced`, идемпотентно по ключу `learn.introduce:<key>`, `409 limit_reached` при превышении остатка, `400` для слов без карточки в `kb`); `GET /learn/placement?page=N` (50 слов `lexemeId, lemma, pos`, `hasMore`); `POST /learn/placement {known, shown, key}` → `{marked, next}` (`more` при ≥ 80 % «знаю» на странице, `stop` при < 50 %, иначе `learn`; события `word.known_at_start`); `GET/PUT /learn/preferences` (`newWordsMode`, `newWordsPerDay`, `campCoverage`; границы 1–30 и 0,90–0,99, `400` с русским текстом); событие `word.learned` при переходе в `learned` в `ReviewCard`.

- [ ] **Step 1: Падающие тесты** с поддельным хранилищем (образец — `exercises_match_test.go` и `words_test.go`; методы фейка в `learn_fake_test.go`): блок (порядок по частоте, повторное знакомство первым, слова с карточкой и без перевода отсутствуют, слово без примера пропускается, `kb` пуст → `404 no_words`, не 500), `introduce` (карточки и статусы, событие, повтор по ключу, лимит → 409, слово без `kb` → 400, чужой/несуществующий номер → 400), `status` (причина и лимит по данным: просрочено, точность), `placement` (страница, `known` создаёт `known_at_start` без карточек, правило `next`, повтор по ключу), `preferences` (значения по умолчанию, границы, `PUT` сохраняет), `ReviewCard` (оценка 3 и 4 → `learned` + `word.learned` один раз; оценки 1 и 2 → нет; нет строки → `learned/card`; повтор по ключу события не удваивает), часовой пояс границы суток (23:50 и 00:10).
- [ ] **Step 2:** `go test ./internal/httpapi -run 'Learn|ReviewLearned'` → FAIL.
- [ ] **Step 3:** Расширить контракт, перегенерировать (`go generate ./internal/api`, `npm run gen:api`), реализовать обработчики; ошибки чтения `kb` в `/learn/block` дают `404 no_words`, остальные ошибки базы — 500; блок собирается так: повторное знакомство, затем кандидаты `LearnCandidates` (до 80), `PickBlock` с проверкой примеров по одному запросу на слово; звук — `https://commons.wikimedia.org/wiki/Special:FilePath/<audio_ref>` (с экранированием имени файла).
- [ ] **Step 4:** `go vet ./... && go test -race ./...` → PASS.
- [ ] **Step 5: Commit** `git commit -m "Знакомство: эндпоинты /learn/* и перевод слова в «изучено» при повторении"`.

---

### Task 4: Экраны «Знакомство», быстрый тест, «Сегодня», настройки

**Files:**
- Create: `web/src/features/learn/learn.ts` (запросы), `LearnPage.tsx`, `WordCard.tsx`, `PlacementPage.tsx`, `LearnPrefs.tsx` и тесты рядом; `web/e2e/learn.spec.ts`
- Modify: маршруты (`web/src/App.tsx` или файл маршрутов; найти `grep -rn "path=\"/cards\"" web/src`), `web/src/features/today/TodayPage.tsx` (карточка «Знакомство»), `web/src/features/settings/SettingsPage.tsx` (блок «Обучение»), `web/e2e/mock-api.ts` (подмена `/learn/*`), `DESIGN.md`
- Skills: перед компонентами вызвать `impeccable`, `animate` и `emil-design-eng`, после — `web-design-guidelines` и `break-ui`.

**Interfaces:**
- Consumes: `GET /learn/status`, `GET /learn/block`, `POST /learn/introduce`, `GET/POST /learn/placement`, `GET/PUT /learn/preferences` (Task 3).
- Produces: `/learn` (блок слов: шаги по словам, «Прослушать» с запасным `speechSynthesis`, примеры с выделением слова, «Повторное знакомство», после последнего слова `introduce` и переход на `/cards`, причина лимита одной строкой), `/learn/placement` (50 слов, кнопки «Знаю» и «Не знаю», «Не знаю» по умолчанию, итог и следующий шаг по `next`), карточка «Знакомство» на «Сегодня» (слов сегодня и причина, кнопки «Начать знакомство» и «Быстрый тест»), блок «Обучение» в настройках (режим, число в день, покрытие; сохранение).

- [ ] **Step 1: Падающие тесты Vitest:** `LearnPage` (показывает слово, перевод, IPA с `lang="en-fonipa"`, примеры с `<mark>`, кнопка звука только если есть источник звука или синтез речи; шаги вперёд и назад клавишами; после последнего слова вызывается `introduce` со всеми номерами и ключом, затем переход на `/cards`; `404 no_words` → «Слов для знакомства пока нет» без ошибки; лимит исчерпан → «На сегодня всё» с причиной; тексты из базы только текстом), `PlacementPage` (по умолчанию «Не знаю»; отметка «Знаю» меняется клавишей; отправка с `known` и `shown`; `next = more` подгружает следующую страницу, `stop` и `learn` показывают итог), `TodayPage` (карточка «Знакомство» с числом и причиной; нет `kbReady` — карточки нет), `LearnPrefs` (значения по умолчанию, границы, сохранение).
- [ ] **Step 2:** `cd web && npx vitest run src/features/learn src/features/today src/features/settings` → FAIL.
- [ ] **Step 3:** Реализовать по `DESIGN.md` (пиксельные рамки, лисёнок, спокойный текст); `WordCard`: слово 28px/600, часть речи и уровень, IPA, переводы 18px/600 (`lang="ru"`), формы и синонимы 14px приглушённым, примеры с переводом и атрибуцией Tatoeba; фокус и клавиши (стрелки, Enter), `aria-live` для шага; отправка один раз (защита от двойного нажатия).
- [ ] **Step 4:** `npx vitest run && npx tsc -b && npm run lint` и `npm run e2e -- learn` (мышь, клавиатура, «уменьшить движение», светлая и тёмная темы, 1440/1024/390) → PASS; снимки смотреть глазами.
- [ ] **Step 5: Commit** `git commit -m "Знакомство: экраны слова, быстрый тест, карточка на «Сегодня» и настройки обучения"`.

---

### Task 5: Живая проверка и документация

**Files:**
- Modify: `CLAUDE.md`, `DESIGN.md`, спек (расхождения)

- [ ] **Step 1: Песочница.** `goose up` до `00015` на `english_pass_sandbox`; сервер на порту 8082; временный пароль пользователю песочницы (файл удалить); пройти: `GET /learn/status`, `GET /learn/block`, `POST /learn/introduce`, оценка карточки «хорошо» → слово `learned` (SQL), быстрый тест, `preferences`; проверить время ответа `GET /learn/block` (цель ≤ 150 мс) и то, что при пустых `kb_*` ответ `404 no_words`.
- [ ] **Step 2: Документация и проверки.** `CLAUDE.md`: этап A (таблицы, эндпоинты, правила блока, формула темпа, что проверено и что НЕТ, ПОРЯДОК ВНЕДРЕНИЯ: `pg_dump` → `goose up` (00015) → сервер), известные ограничения; `go vet ./... && go test -race ./... && golangci-lint run`, в `web/` `npx vitest run && npx tsc -b && npm run lint && npm run e2e`.
- [ ] **Step 3: Commit** `git commit -m "Знакомство: живая проверка и документация"`.

---

## Self-review

- Покрытие спека (раздел «Уточнения» и этап A): состояние изучения и перенос старых карточек (Task 1), подбор блока и разнообразие (Task 2), адаптивный темп и ручной режим (Task 2, 3), `introduce`, `status`, быстрый тест, настройки (Task 3), экраны и «Сегодня» (Task 4), живая проверка (Task 5). «Привал» — этап B, отдельный план.
- Имена: `PickBlock`, `NewWordsToday`, `LearnCandidates`, `LearnMarkLearned`, `word_learning`, `learn_preferences` одинаковы во всех задачах.
- Риски: скорость `LearnCandidates` на 9 779 словах (индексы на `word_learning (user_id, status)` и `kb_lexeme_stat.freq_rank`; измерить в Task 5); проверка примеров по слову стоит ≈ 4 мс, при 80 кандидатах ≤ 0,3 с в худшем случае.
