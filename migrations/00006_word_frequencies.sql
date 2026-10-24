-- +goose Up

-- Частотный список слов: сколько раз слово встретилось в корпусе и его место в списке (1 — самое частое).
-- Хранится как есть, по написанию (формы слов: run, runs, running). Частота слова словаря считается при чтении
-- по его формам, поэтому порядок импортов и новые слова ничего не требуют перезагружать.
CREATE TABLE word_frequencies (
    source_id bigint  NOT NULL REFERENCES data_sources (id) ON DELETE RESTRICT,
    word      text    NOT NULL CHECK (length(word) BETWEEN 1 AND 64),
    count     bigint  NOT NULL CHECK (count > 0),
    rank      integer NOT NULL CHECK (rank > 0),
    PRIMARY KEY (source_id, word),
    UNIQUE (source_id, rank)
);

-- +goose Down
-- ВНИМАНИЕ: откат удаляет частотные списки; их можно загрузить заново командой import.
DROP TABLE word_frequencies;
