-- name: UpsertWordFrequency :exec
INSERT INTO word_frequencies (source_id, word, count, rank)
VALUES ($1, $2, $3, $4)
ON CONFLICT (source_id, word) DO UPDATE SET count = EXCLUDED.count, rank = EXCLUDED.rank;

-- name: ListWordFrequencies :many
-- Места слов в списке частот. Берётся последний завершённый импорт источника с таким именем:
-- недогруженная версия и старые версии не подмешиваются.
SELECT wf.word, wf.rank
FROM word_frequencies wf
WHERE wf.source_id = (
        SELECT s.id FROM data_sources s
        WHERE s.name = sqlc.arg(source_name) AND s.imported_at IS NOT NULL
        ORDER BY s.imported_at DESC, s.id DESC
        LIMIT 1
      )
  AND wf.word = ANY(sqlc.arg(words)::text[]);
