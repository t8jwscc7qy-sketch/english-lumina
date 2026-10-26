package kb

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Путь считается при старте пакета: тесты не должны зависеть от рабочего каталога.
var samplePath, _ = filepath.Abs("testdata/kaikki-sample.jsonl")

func sampleLines(t *testing.T) [][]byte {
	t.Helper()
	f, err := os.Open(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out [][]byte
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		out = append(out, append([]byte(nil), sc.Bytes()...))
	}
	return out
}

func lineFor(t *testing.T, word, lang string) []byte {
	t.Helper()
	for _, l := range sampleLines(t) {
		var probe struct {
			Word     string `json:"word"`
			LangCode string `json:"lang_code"`
		}
		if err := json.Unmarshal(l, &probe); err == nil && probe.Word == word && probe.LangCode == lang {
			return l
		}
	}
	t.Fatalf("в фикстуре нет %s/%s", word, lang)
	return nil
}

func TestParseEntry(t *testing.T) {
	e, err := ParseEntry(lineFor(t, "dad", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Word != "dad" || e.Pos != "noun" || e.LangCode != "en" {
		t.Fatalf("запись: %+v", e)
	}
	if len(e.Senses) != 1 || !slices.Equal(e.Senses[0].Glosses, []string{"A father."}) || !slices.Equal(e.Senses[0].Tags, []string{"informal"}) {
		t.Fatalf("значения: %+v", e.Senses)
	}
	if len(e.Sounds) != 3 || e.Sounds[0].IPA != "/dæd/" || !slices.Contains(e.Sounds[0].Tags, "US") || e.Sounds[2].Audio != "En-us-dad.ogg" {
		t.Fatalf("звуки: %+v", e.Sounds)
	}
	if len(e.Forms) != 1 || e.Forms[0].Form != "dads" || !slices.Contains(e.Forms[0].Tags, "plural") {
		t.Fatalf("формы: %+v", e.Forms)
	}
	var ru []string
	for _, tr := range e.Translations {
		if tr.Code == "ru" {
			ru = append(ru, tr.Word)
			if tr.Sense != "father" {
				t.Fatalf("у перевода нет значения: %+v", tr)
			}
		}
	}
	if !slices.Equal(ru, []string{"па́па", "бать", "батю́шка"}) {
		t.Fatalf("русские переводы %v (ударения сохраняются как есть)", ru)
	}
	if !slices.Equal(e.Synonyms, []string{"father", "pa"}) {
		t.Fatalf("синонимы %v", e.Synonyms)
	}
	var archaic bool
	for _, tr := range e.Translations {
		if tr.Word == "батю́шка" && slices.Contains(tr.Tags, "archaic") {
			archaic = true
		}
	}
	if !archaic {
		t.Fatal("пометка archaic у перевода потеряна")
	}
}

func TestParseEntrySenseLevelTranslationsAndTags(t *testing.T) {
	e, err := ParseEntry(lineFor(t, "blow", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Senses) != 2 || !slices.Equal(e.Senses[1].Tags, []string{"vulgar", "slang"}) {
		t.Fatalf("значения: %+v", e.Senses)
	}
	if len(e.Senses[1].Translations) != 1 || e.Senses[1].Translations[0].Word != "соса́ть" || !slices.Contains(e.Senses[1].Translations[0].Tags, "vulgar") {
		t.Fatalf("переводы значения: %+v", e.Senses[1].Translations)
	}
	if len(e.Forms) != 4 {
		t.Fatalf("формы: %+v", e.Forms)
	}
}

func TestParseEntryAntonymsAndSynonyms(t *testing.T) {
	e, err := ParseEntry(lineFor(t, "thin", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.Antonyms, []string{"thick", "fat"}) || !slices.Equal(e.Synonyms, []string{"slim"}) {
		t.Fatalf("связи: %+v / %+v", e.Antonyms, e.Synonyms)
	}
	if e.Pos != "adj" {
		t.Fatalf("pos %q", e.Pos)
	}
}

func TestParseEntryRawGlossesFallback(t *testing.T) {
	e, err := ParseEntry(lineFor(t, "nothing", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Senses) != 1 || !slices.Equal(e.Senses[0].Glosses, []string{"(pronoun) not anything"}) {
		t.Fatalf("при отсутствии glosses берутся raw_glosses: %+v", e.Senses)
	}
}

func TestParseEntryEmptyFields(t *testing.T) {
	e, err := ParseEntry(lineFor(t, "empty", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Word != "empty" || len(e.Senses) != 0 || len(e.Sounds) != 0 || len(e.Forms) != 0 || len(e.Translations) != 0 {
		t.Fatalf("запись: %+v", e)
	}
}

func TestParseEntryBrokenJSON(t *testing.T) {
	for _, bad := range []string{``, `{`, `not json`, `{"word": 5}`, `[]`} {
		if _, err := ParseEntry([]byte(bad)); err == nil {
			t.Errorf("%q: ожидали ошибку", bad)
		}
	}
}

func TestWantedBothSpacings(t *testing.T) {
	lemmas := map[string]bool{"dad": true, "clothes": true}
	// «dad» записан с пробелами после двоеточия, «clothes» без них (фикстура содержит оба стиля)
	if !Wanted(lineFor(t, "dad", "en"), lemmas) || !Wanted(lineFor(t, "clothes", "en"), lemmas) {
		t.Fatal("английские слова из списка должны отбираться при любом стиле пробелов")
	}
}

func TestWantedSkipsOtherLanguageAndUnknownWord(t *testing.T) {
	lemmas := map[string]bool{"dad": true}
	if Wanted(lineFor(t, "dad", "de"), lemmas) {
		t.Fatal("немецкая запись не нужна")
	}
	if Wanted(lineFor(t, "zzyzx", "en"), lemmas) {
		t.Fatal("слова нет в списке лемм")
	}
	if Wanted([]byte(`not json`), lemmas) {
		t.Fatal("битая строка не отбирается")
	}
}

func TestParseEntryEtymologyNumber(t *testing.T) {
	e, err := ParseEntry([]byte(`{"word":"school","pos":"noun","lang_code":"en","etymology_number":2}`))
	if err != nil || e.EtymNo != 2 {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestParseEntryEtymologyNumberAsString(t *testing.T) {
	for in, want := range map[string]int{`"2"`: 2, `2`: 2, `"x"`: 0, `null`: 0, `1.5`: 0} {
		e, err := ParseEntry([]byte(`{"word":"w","pos":"noun","lang_code":"en","etymology_number":` + in + `}`))
		if err != nil || e.EtymNo != want {
			t.Errorf("%s: %+v %v, ждали %d", in, e, err, want)
		}
	}
}
