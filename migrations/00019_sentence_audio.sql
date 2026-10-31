-- +goose Up
-- Записи произношения предложений Tatoeba: файлы не хранятся, браузер берёт их по адресу tatoeba.org/audio/download/<audio_id>.
-- Автор и лицензия нужны для атрибуции. Хранятся только записи с разрешённой лицензией (решает источник tatoeba-audio).
CREATE TABLE sentence_audio (
    audio_id        bigint PRIMARY KEY CHECK (audio_id > 0),
    sentence_tatoeba_id bigint NOT NULL CHECK (sentence_tatoeba_id > 0), -- номер предложения на tatoeba.org
    username        text   NOT NULL CHECK (length(username) BETWEEN 1 AND 64),
    license         text   NOT NULL CHECK (length(license) BETWEEN 1 AND 32),
    attribution_url text,
    source_id       bigint NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT
);
CREATE INDEX sentence_audio_sentence ON sentence_audio (sentence_tatoeba_id);

-- +goose Down
DROP TABLE sentence_audio;
