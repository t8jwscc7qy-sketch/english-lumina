-- name: GetLexemeByPair :one
SELECT * FROM lexemes WHERE lemma = $1 AND pos = $2;

-- name: InsertCard :one
-- При повторном добавлении того же слова строк не возвращает (ErrNoRows): карточка уже есть.
INSERT INTO cards (user_id, lexeme_id, due, stability, difficulty, scheduled_days, reps, lapses, state, last_review, remaining_steps)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id, lexeme_id) DO NOTHING
RETURNING id;

-- name: GetCardByLexeme :one
SELECT * FROM cards WHERE user_id = $1 AND lexeme_id = $2;

-- name: GetCardForUpdate :one
-- Карточка пользователя со словом; строка блокируется до конца транзакции, чтобы две оценки не наложились.
SELECT c.*, l.lemma, l.pos
FROM cards c
JOIN lexemes l ON l.id = c.lexeme_id
WHERE c.id = $1 AND c.user_id = $2
FOR UPDATE OF c;

-- name: UpdateCardState :exec
UPDATE cards SET
    due             = $2,
    stability       = $3,
    difficulty      = $4,
    scheduled_days  = $5,
    reps            = $6,
    lapses          = $7,
    state           = $8,
    last_review     = $9,
    remaining_steps = $10,
    updated_at      = now()
WHERE id = $1;

-- name: NextDueCard :one
-- Следующая карточка очереди: самый ранний срок среди просроченных. Уровень слова — самый низкий из оценок источников.
SELECT c.*, l.lemma, l.pos,
       (SELECT min(lc.level) FROM lexeme_cefr lc WHERE lc.lexeme_id = l.id)::text AS level
FROM cards c
JOIN lexemes l ON l.id = c.lexeme_id
WHERE c.user_id = sqlc.arg(user_id) AND c.due <= sqlc.arg(now)::timestamptz
ORDER BY c.due, c.id
LIMIT 1;

-- name: CountCards :one
SELECT count(*) AS total,
       count(*) FILTER (WHERE due <= sqlc.arg(now)::timestamptz) AS due
FROM cards
WHERE user_id = sqlc.arg(user_id);

-- name: ListCardLexemeIDs :many
-- Какие из слов уже есть у пользователя в карточках (для признака inCards в поиске).
SELECT lexeme_id FROM cards WHERE user_id = sqlc.arg(user_id) AND lexeme_id = ANY(sqlc.arg(ids)::bigint[]);
