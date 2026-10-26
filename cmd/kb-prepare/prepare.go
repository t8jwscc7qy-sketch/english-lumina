package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/kb"
)

// maxLine — потолок длины одной записи JSONL: самые большие записи Викисловаря (a, the, go) занимают мегабайты.
const maxLine = 128 << 20

// Stats — счётчики прохода: всего строк, английских, отобрано.
type Stats struct {
	Lines    int64
	English  int64
	Selected int64
}

// Prepare читает gzip-поток JSONL Kaikki построчно и пишет в out (как есть, по строке) записи английских слов из
// lemmas. Ошибки gzip и обрыв архива возвращаются, отмена контекста прерывает проход.
func Prepare(ctx context.Context, in io.Reader, lemmas map[string]bool, out io.Writer) (Stats, error) {
	var st Stats
	zr, err := gzip.NewReader(in)
	if err != nil {
		return st, fmt.Errorf("архив: %w", err)
	}
	defer func() { _ = zr.Close() }()
	if err := ctx.Err(); err != nil {
		return st, err
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), maxLine)
	w := bufio.NewWriter(out)
	for sc.Scan() {
		st.Lines++
		if st.Lines%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return st, err
			}
		}
		line := sc.Bytes()
		if !kb.IsEnglishLine(line) {
			continue
		}
		st.English++
		if !kb.Wanted(line, lemmas) {
			continue
		}
		st.Selected++
		if _, err := w.Write(line); err != nil {
			return st, err
		}
		if err := w.WriteByte('\n'); err != nil {
			return st, err
		}
	}
	if err := sc.Err(); err != nil {
		return st, fmt.Errorf("чтение архива: %w", err)
	}
	return st, w.Flush()
}
