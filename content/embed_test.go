package content

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/constructions"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/course"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/library"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sheets"
)

// Встроенные шпаргалки проходят проверку схемы: битый файл ловится тестом, а не в рантайме.
func TestEmbeddedSheetsAreValid(t *testing.T) {
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("нет ни одной шпаргалки")
	}
}

// Каждая лемма привязки существует в загруженных словах (проверка на настоящих данных; без базы пропускается).
func TestSheetWordKeysExistInLexemes(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("нет DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skip("база недоступна")
	}
	defer pool.Close()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM lexemes`).Scan(&n); err != nil || n == 0 {
		t.Skip("слова не загружены")
	}
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		keys := s.WordKeys()
		if len(keys) == 0 {
			continue
		}
		poss := []string{}
		for _, b := range s.Blocks {
			if b.Type == sheets.BlockTable && b.Bind == sheets.BindWord {
				poss = append(poss, b.Pos...)
			}
		}
		rows, err := pool.Query(ctx, `SELECT DISTINCT lemma FROM lexemes WHERE lemma = ANY($1) AND (cardinality($2::text[]) = 0 OR pos = ANY($2::text[]))`, keys, poss)
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for rows.Next() {
			var l string
			if err := rows.Scan(&l); err != nil {
				t.Fatal(err)
			}
			have[l] = true
		}
		rows.Close()
		var missing []string
		for _, k := range keys {
			if !have[k] {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("шпаргалка %s: нет в словах: %v", s.ID, missing)
		}
	}
}

// Каждая конструкция, которую умеет находить `constructions.Detect`, имеет пояснение, а ссылки ведут на настоящие шпаргалки.
func TestEmbeddedConstructionsAreValidAndComplete(t *testing.T) {
	infos, err := constructions.LoadAll(Constructions)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range constructions.Order {
		if _, ok := infos[id]; !ok {
			t.Errorf("у конструкции %s нет пояснения", id)
		}
	}
	for id := range infos {
		if !slices.Contains(constructions.Order, id) {
			t.Errorf("пояснение %s без правила поиска", id)
		}
	}
	all, err := sheets.LoadAll(Sheets)
	if err != nil {
		t.Fatal(err)
	}
	for id, info := range infos {
		if info.SheetID == "" {
			continue
		}
		if !slices.ContainsFunc(all, func(s sheets.Sheet) bool { return s.ID == info.SheetID }) {
			t.Errorf("%s: шпаргалки %q нет", id, info.SheetID)
		}
	}
}

// Авторские тексты проходят проверку структуры; словарь уровня проверяется по базе знаний (без базы пропускается).
func TestEmbeddedTextsAreValid(t *testing.T) {
	texts, err := library.LoadAll(Texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) < 40 {
		t.Fatalf("текстов %d, ждали не меньше 40", len(texts))
	}
}

func TestEmbeddedTextsMatchBandVocabulary(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("нет DATABASE_URL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skip("база недоступна")
	}
	defer pool.Close()
	q := db.New(pool)
	texts, err := library.LoadAll(Texts)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[int]map[string]bool{}
	for _, tx := range texts {
		if allowed[tx.Band] == nil {
			rows, err := q.LibraryBandLexemes(ctx, int32(tx.Band))
			if err != nil || len(rows) == 0 {
				t.Skip("база знаний не собрана")
			}
			ws := make([]library.BandWord, len(rows))
			for i, r := range rows {
				ws[i] = library.BandWord{Lemma: r.Lemma, Pos: r.Pos}
			}
			allowed[tx.Band] = library.AllowedForms(ws)
		}
		if probs := tx.CheckVocabulary(allowed[tx.Band]); len(probs) > 0 {
			t.Errorf("%s: слова вне словаря уровня %d: %v", tx.ID, tx.Band, probs)
		}
	}
}

func TestEmbeddedTextsHaveNoStyleIssues(t *testing.T) {
	texts, err := library.LoadAll(Texts)
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range texts {
		for _, is := range tx.CheckStyle() {
			t.Errorf("%s: %s", tx.ID, is)
		}
	}
}

func TestEmbeddedCourseIsValid(t *testing.T) {
	c, err := course.Load(Course)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.Ordered()); n < 3 {
		t.Fatalf("уроков %d, ждали не меньше 3", n)
	}
	for i := range c.Ordered() {
		drills := course.Drills(c, i)
		if len(drills) < 4 {
			t.Errorf("урок %d: заданий меньше 4", i+1)
		}
		for _, d := range drills {
			if d.Kind != course.KindGap || d.Answer == "I" {
				continue
			}
			for _, o := range d.Options {
				if o != "I" && (o[:1] == strings.ToUpper(o[:1])) != (d.Answer[:1] == strings.ToUpper(d.Answer[:1])) {
					t.Errorf("урок %d: регистр подсказывает ответ в %q: %v", i+1, d.Text, d.Options)
				}
			}
		}
	}
}
