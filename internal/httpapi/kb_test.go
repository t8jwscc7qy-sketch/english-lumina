package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

// KbGetTeachingCards отдаёт заданные тестом карточки запрошенных слов (как запрос: только они).
func (f *fakeStore) KbGetTeachingCards(_ context.Context, ids []int64) ([]db.KbGetTeachingCardsRow, error) {
	if f.failKb {
		return nil, errors.New(`relation "kb_teaching_card" does not exist`)
	}
	var out []db.KbGetTeachingCardsRow
	for _, id := range ids {
		if p, ok := f.kbCards[id]; ok {
			out = append(out, db.KbGetTeachingCardsRow{LexemeID: id, Payload: p})
		}
	}
	return out, nil
}

// KbListChosenTranslations отдаёт заданные тестом переводы запрошенных слов, основной первым.
func (f *fakeStore) KbListChosenTranslations(_ context.Context, arg db.KbListChosenTranslationsParams) ([]db.KbListChosenTranslationsRow, error) {
	if f.failKb {
		return nil, errors.New(`relation "kb_translation" does not exist`)
	}
	var out []db.KbListChosenTranslationsRow
	for _, id := range arg.LexemeIds {
		for i, text := range f.kbTranslations[id] {
			out = append(out, db.KbListChosenTranslationsRow{LexemeID: id, Text: text, Rank: int32(i + 1)}) //nolint:gosec // малое число
		}
	}
	return out, nil
}

func kbWordsEnv(t *testing.T) (env, string) {
	t.Helper()
	e := newEnv(t)
	e.store.lexemes = []db.SearchLexemesRow{{ID: 1, Lemma: "dad", Pos: "noun", Level: "A1"}, {ID: 2, Lemma: "zebra", Pos: "noun", Level: "A2"}}
	return e, login(t, e)
}

func TestWordsIncludeKbFields(t *testing.T) {
	e, token := kbWordsEnv(t)
	e.store.kbCards = map[int64]json.RawMessage{
		1: json.RawMessage(`{"lemma":"dad","pos":"noun","ipa":"dæd","translations":["папа","бать"],"synonyms":["father","pa"]}`),
	}
	rec := e.do(http.MethodGet, wordsPath("d"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	got := wordsByLemma(t, rec.Body.String())
	dad := got["dad"]
	if dad.Ipa == nil || *dad.Ipa != "dæd" || dad.Translations == nil || !slices.Equal(*dad.Translations, []string{"папа", "бать"}) ||
		dad.Synonyms == nil || !slices.Equal(*dad.Synonyms, []string{"father", "pa"}) {
		t.Fatalf("поля базы знаний у dad: %+v", dad)
	}
	zebra := got["zebra"]
	if zebra.Ipa != nil || zebra.Translations != nil || zebra.Synonyms != nil {
		t.Fatalf("у слова без карточки полей нет: %+v", zebra)
	}
}

func TestWordsWithoutKbUnchanged(t *testing.T) {
	e, token := kbWordsEnv(t)
	rec := e.do(http.MethodGet, wordsPath("d"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	for _, key := range []string{`"ipa"`, `"translations"`, `"synonyms"`} {
		if strings.Contains(rec.Body.String(), key) {
			t.Fatalf("без базы знаний в ответе есть %s: %s", key, rec.Body)
		}
	}
}

func TestWordsKbBadPayloadIgnored(t *testing.T) {
	e, token := kbWordsEnv(t)
	e.store.kbCards = map[int64]json.RawMessage{1: json.RawMessage(`не json`), 2: json.RawMessage(`{"translations":[]}`)}
	rec := e.do(http.MethodGet, wordsPath("d"), "", withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("битая карточка не должна ронять поиск: статус %d, тело %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), `"translations"`) {
		t.Fatalf("пустые и битые карточки полей не дают: %s", rec.Body)
	}
}

func TestWordsKbErrorDegrades(t *testing.T) {
	e, token := kbWordsEnv(t)
	e.store.failKb = true
	rec := e.do(http.MethodGet, wordsPath("d"), "", withCookie(token))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"dad"`) {
		t.Fatalf("без базы знаний поиск работает: статус %d, тело %s", rec.Code, rec.Body)
	}
}

func TestMatchPrefersKbTranslation(t *testing.T) {
	e, token := matchEnv(t)
	e.store.kbTranslations = map[int64][]string{1: {"плод"}} // слово apple (первое в наборе): в старом словаре «яблоко»
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Options == nil || !slices.Contains(*ex.Options, "плод") || slices.Contains(*ex.Options, "яблоко") {
		t.Fatalf("ждали перевод из базы знаний «плод» вместо «яблоко»: %+v", ex.Options)
	}
}

func TestMatchUsesSecondKbTranslationWhenFirstUnfit(t *testing.T) {
	e, token := matchEnv(t)
	e.store.kbTranslations = map[int64][]string{1: {"красный плод", "плод"}} // первый — словосочетание, для плиток не годится
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Options == nil || !slices.Contains(*ex.Options, "плод") {
		t.Fatalf("вторым переводом должен подойти «плод»: %+v", ex.Options)
	}
}

func TestMatchKbErrorDegradesToLegacy(t *testing.T) {
	e, token := matchEnv(t)
	e.store.failKb = true
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("без базы знаний «Сопоставление» работает по старому словарю: статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Options == nil || !slices.Contains(*ex.Options, "яблоко") {
		t.Fatalf("ждали перевод из старого словаря: %+v", ex.Options)
	}
}
