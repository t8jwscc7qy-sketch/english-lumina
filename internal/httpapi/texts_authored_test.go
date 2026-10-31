package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/content"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/library"
)

func mkEntry(id int64, tokens []string) authoredEntry {
	return authoredEntry{dbID: id, tokens: [][]string{tokens}}
}

func TestPickAuthoredPrefersWindowAndSkipsRead(t *testing.T) {
	srv := &authServer{rnd: func(int64) int64 { return 0 }}
	known := map[string]bool{"cat": true, "dog": true, "run": true, "big": true}
	// покрытие: e1 = 4/5 = 0,8; e2 = 4/4 = 1,0 (вне допустимого); e3 = 3/4 = 0,75; e4 = 2/4 = 0,5
	entries := []authoredEntry{
		mkEntry(1, []string{"cat", "dog", "run", "big", "xxx"}),
		mkEntry(2, []string{"cat", "dog", "run", "big"}),
		mkEntry(3, []string{"cat", "dog", "run", "yyy"}),
		mkEntry(4, []string{"cat", "dog", "aaa", "bbb"}),
	}
	got, cov, ok := srv.pickAuthored(entries, map[int64]bool{}, known, 0.80)
	if !ok || got.dbID != 1 || cov < 0.79 || cov > 0.81 {
		t.Fatalf("в окне цели 0,80 нужен текст 1: %d %v %v", got.dbID, cov, ok)
	}
	// Прочитанный пропускается; остаётся ближайший к цели в допустимых границах (0,75).
	got, _, ok = srv.pickAuthored(entries, map[int64]bool{1: true}, known, 0.85)
	if !ok || got.dbID != 3 {
		t.Fatalf("после прочтения 1 ближайший допустимый — 3, а не %d (ok=%v)", got.dbID, ok)
	}
	if _, _, ok := srv.pickAuthored(entries, map[int64]bool{1: true, 2: true, 3: true}, known, 0.85); ok {
		t.Fatal("остался только текст с понятностью 0,5: он не подходит")
	}
	if _, _, ok := srv.pickAuthored(nil, nil, known, 0.85); ok {
		t.Fatal("без текстов выбирать нечего")
	}
}

func TestWithoutNamesRemovesHeroesAndPossessives(t *testing.T) {
	got := withoutNames([]string{"Tom", "has", "Kate's", "dog", "and", "Anna"}, []string{"Tom", "Kate"})
	if !slices.Equal(got, []string{"has", "dog", "and", "Anna"}) {
		t.Fatalf("%v", got)
	}
}

// authoredEnv — пользователь, который знает все слова всех авторских текстов, кроме целевых: любой авторский текст подходит.
func authoredEnv(t *testing.T) (env, string, []library.Text) {
	t.Helper()
	all, err := library.LoadAll(content.Texts)
	if err != nil {
		t.Fatal(err)
	}
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.wl = map[int64]fakeWord{}
	targets := map[string]bool{}
	for _, tx := range all {
		for _, tg := range tx.Targets {
			for f := range tg.Forms() {
				targets[f] = true
			}
		}
	}
	seen := map[string]bool{}
	id := int64(100)
	for _, tx := range all {
		for _, s := range tx.Sentences {
			for _, tok := range withoutNames(s.Tokens(), tx.Names) {
				w := lowerTokenForTest(tok)
				if seen[w] || targets[w] {
					continue
				}
				seen[w] = true
				id++
				e.store.addLexeme(id, w, "noun", "A1")
				e.store.wl[id] = fakeWord{status: "learned", source: "lesson"}
			}
		}
	}
	// Подсказки для целевых слов (лемма и карточка базы знаний).
	for i, tx := range all {
		for j, tg := range tx.Targets {
			payload, _ := json.Marshal(map[string]any{"lemma": tg.Lemma, "translations": []string{"перевод-" + tg.Lemma}})
			e.store.textHints = append(e.store.textHints, db.TextWordHintsRow{LexemeID: int64(5000 + i*10 + j), Lemma: tg.Lemma, Pos: tg.Pos, Payload: payload})
		}
	}
	return e, token, all
}

func lowerTokenForTest(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func TestTextNextPrefersAuthoredTextWithoutTatoebaAttribution(t *testing.T) {
	e, token, all := authoredEnv(t)
	e.store.learnPrefs = &db.LearnPreference{UserID: 1, NewWordsMode: "auto", NewWordsPerDay: 12, CampCoverage: 0.95, TextCoverage: 0.85}
	titles := map[string]bool{}
	for _, tx := range all {
		titles[tx.Title] = true
	}
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	if !titles[text.Title] {
		t.Fatalf("выдан не авторский текст: %q", text.Title)
	}
	for _, s := range text.Sentences {
		if s.TatoebaId != nil || s.Author != nil || s.En == "" || s.Ru == "" || len(s.Tokens) == 0 {
			t.Fatalf("у авторского предложения нет атрибуции Tatoeba, но есть тексты: %+v", s)
		}
	}
	if text.Coverage < 0.70 || text.Coverage > 0.97 || len(text.Targets) < 3 {
		t.Fatalf("понимание %v, целевых %d", text.Coverage, len(text.Targets))
	}
	hasTarget := false
	for _, h := range text.Hints {
		hasTarget = hasTarget || (h.Target && len(h.Translations) == 1)
	}
	if !hasTarget {
		t.Fatalf("у целевых слов должны быть подсказки: %+v", text.Hints)
	}
}

func TestAuthoredTextIsServedOnceThenNextOne(t *testing.T) {
	e, token, _ := authoredEnv(t)
	first, _, _ := nextText(t, e, token, "")
	again, _, _ := nextText(t, e, token, "")
	if again.Id != first.Id {
		t.Fatal("неоценённый авторский текст отдаётся повторно")
	}
	if rec := e.do(http.MethodPost, textRatePath, rateBody(first.Id, "ok"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("оценка: %d %s", rec.Code, rec.Body)
	}
	seen := map[int64]bool{first.Id: true}
	for i := 0; i < 10; i++ { // прочитанные не повторяются
		next, code, body := nextText(t, e, token, "")
		if code != http.StatusOK {
			t.Fatalf("%d %s", code, body)
		}
		if seen[next.Id] {
			t.Fatalf("текст %d уже выдавался и оценён", next.Id)
		}
		seen[next.Id] = true
		if rec := e.do(http.MethodPost, textRatePath, rateBody(next.Id, "ok"), withCookie(token)); rec.Code != http.StatusOK {
			t.Fatal(rec.Code)
		}
	}
}

func TestAuthoredFallsBackToStitchingWhenNothingFits(t *testing.T) {
	e, token := textsEnv(t, 25) // слова «kwNN» не из словарей авторских текстов: ни один не подходит
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	if len(text.Sentences) == 0 || text.Sentences[0].TatoebaId == nil {
		t.Fatalf("запасной вариант — склейка предложений Tatoeba: %+v", text.Sentences[0])
	}
}

func TestAuthoredTextsAreWrittenOnce(t *testing.T) {
	e, token, all := authoredEnv(t)
	nextText(t, e, token, "?new=1")
	nextText(t, e, token, "?new=1")
	authored := 0
	for _, tx := range e.store.texts {
		if tx.Kind == "authored" {
			authored++
		}
	}
	if authored != len(all) {
		t.Fatalf("авторских текстов в базе %d, ждали %d (запись идемпотентна)", authored, len(all))
	}
}
