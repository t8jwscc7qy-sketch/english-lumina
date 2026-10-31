-- name: UpsertSentenceAudio :execrows
-- Запись загружается, только если английское предложение с таким номером есть у нас (остальные языки не нужны).
INSERT INTO sentence_audio (audio_id, sentence_tatoeba_id, username, license, attribution_url, source_id)
SELECT sqlc.arg(audio_id)::bigint, sqlc.arg(sentence_tatoeba_id)::bigint, sqlc.arg(username), sqlc.arg(license), sqlc.narg(attribution_url), sqlc.arg(source_id)::bigint
WHERE EXISTS (SELECT 1 FROM sentences s WHERE s.lang = 'en' AND s.tatoeba_id = sqlc.arg(sentence_tatoeba_id)::bigint)
ON CONFLICT (audio_id) DO NOTHING;

-- name: SentenceAudioFor :many
-- Одна запись на предложение (с наименьшим номером) для номеров предложений Tatoeba; лицензии из списка разрешённых.
SELECT DISTINCT ON (sentence_tatoeba_id) sentence_tatoeba_id, audio_id, username, license, attribution_url
FROM sentence_audio
WHERE sentence_tatoeba_id = ANY(sqlc.arg(ids)::bigint[]) AND license = ANY(sqlc.arg(licenses)::text[])
ORDER BY sentence_tatoeba_id, audio_id;
