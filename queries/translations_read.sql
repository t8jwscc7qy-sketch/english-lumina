-- name: ListTranslationsForWords :many
-- Переводы слов из последнего завершённого импорта словаря (недогруженная версия и старые версии не подмешиваются).
-- Ключ слова: лемма и часть речи словаря (n, v, adj, adv); значения идут по порядку sense_no.
SELECT wt.lemma, wt.pos, wt.sense_no, wt.ru
FROM word_translations wt
WHERE wt.source_id = (
        SELECT s.id FROM data_sources s
        WHERE s.name = sqlc.arg(source_name) AND s.imported_at IS NOT NULL
        ORDER BY s.imported_at DESC, s.id DESC
        LIMIT 1
      )
  AND wt.lemma = ANY(sqlc.arg(lemmas)::text[])
ORDER BY wt.lemma, wt.pos, wt.sense_no, wt.ru;
