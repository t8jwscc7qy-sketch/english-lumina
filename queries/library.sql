-- name: LibraryBandLexemes :many
-- Слова словаря уровня: самые частотные max_n различных лемм (без учёта регистра) и все их части речи.
WITH r AS (
    SELECT lower(l.lemma) AS lemma, min(st.freq_rank) AS rk
    FROM kb_lexeme_stat st
    JOIN lexemes l ON l.id = st.lexeme_id
    WHERE st.freq_rank IS NOT NULL AND l.lemma ~ '^[A-Za-z]+$'
    GROUP BY 1
    ORDER BY 2, 1
    LIMIT sqlc.arg(max_n)::int
)
SELECT l.lemma, l.pos, r.rk AS freq_rank
FROM lexemes l
JOIN r ON r.lemma = lower(l.lemma)
WHERE l.lemma ~ '^[A-Za-z]+$'
ORDER BY r.rk, l.pos;
