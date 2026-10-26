-- name: SheetStates :many
SELECT sheet_id, favorite_at, reviewed_at, opened_at FROM sheet_state WHERE user_id = $1;

-- name: SheetSetFavorite :execrows
-- 1 строка — состояние изменилось (звезда поставлена), 0 — звезда уже стояла.
INSERT INTO sheet_state (user_id, sheet_id, favorite_at)
VALUES (sqlc.arg(user_id), sqlc.arg(sheet_id), sqlc.arg(at)::timestamptz)
ON CONFLICT (user_id, sheet_id) DO UPDATE SET favorite_at = EXCLUDED.favorite_at WHERE sheet_state.favorite_at IS NULL;

-- name: SheetClearFavorite :execrows
UPDATE sheet_state SET favorite_at = NULL WHERE user_id = $1 AND sheet_id = $2 AND favorite_at IS NOT NULL;

-- name: SheetSetReviewed :execrows
-- Отметка «проверено» ставится один раз.
INSERT INTO sheet_state (user_id, sheet_id, reviewed_at)
VALUES (sqlc.arg(user_id), sqlc.arg(sheet_id), sqlc.arg(at)::timestamptz)
ON CONFLICT (user_id, sheet_id) DO UPDATE SET reviewed_at = EXCLUDED.reviewed_at WHERE sheet_state.reviewed_at IS NULL;

-- name: SheetTouchOpened :exec
INSERT INTO sheet_state (user_id, sheet_id, opened_at)
VALUES (sqlc.arg(user_id), sqlc.arg(sheet_id), sqlc.arg(at)::timestamptz)
ON CONFLICT (user_id, sheet_id) DO UPDATE SET opened_at = EXCLUDED.opened_at;

-- name: SheetWordStatuses :many
-- Данные пользователя по словам шпаргалки (по лемме, в любой части речи): состояние изучения, есть ли карточка,
-- сколько раз забыл. У леммы может быть несколько слов (run: глагол и существительное): Go берёт самое «живое».
SELECT l.lemma, w.status AS learning_status, (cd.id IS NOT NULL)::boolean AS has_card, COALESCE(cd.lapses, 0)::int AS lapses
FROM lexemes l
LEFT JOIN word_learning w ON w.lexeme_id = l.id AND w.user_id = sqlc.arg(user_id)
LEFT JOIN cards cd ON cd.lexeme_id = l.id AND cd.user_id = sqlc.arg(user_id)
WHERE l.lemma = ANY(sqlc.arg(lemmas)::text[]);
