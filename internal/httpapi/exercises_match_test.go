package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/exercises"
)

// matchFirst: у остальных тем рейтинг выше, поэтому skill:match (без попыток) выбирается первой.
func matchFirst(e env) {
	formFirst(e)
	e.store.topicRatings[topicKey{1, formTopic}] = rated(formTopic, 1700, 5)
}

type mw struct{ lemma, pos, dictPos, ru string }

var matchWords8 = []mw{
	{"apple", "noun", "n", "яблоко"}, {"house", "noun", "n", "дом"}, {"water", "noun", "n", "вода"},
	{"book", "noun", "n", "книга"}, {"dog", "noun", "n", "собака"}, {"cat", "noun", "n", "кошка"},
	{"run", "verb", "v", "бежать"}, {"big", "adjective", "adj", "большой"},
}

// matchEnv: пул слов уровня A1 и словарь; skill:match выбирается первой.
func matchEnv(t *testing.T, ws ...mw) (env, string) {
	t.Helper()
	e := newExerciseEnv(t)
	token := login(t, e)
	matchFirst(e)
	if len(ws) == 0 {
		ws = matchWords8
	}
	var lex []db.RandomLexemesByLevelRow
	for i, w := range ws {
		lex = append(lex, db.RandomLexemesByLevelRow{ID: int64(i + 1), Lemma: w.lemma, Pos: w.pos})
		e.store.translations = append(e.store.translations, db.ListTranslationsForWordsRow{Lemma: w.lemma, Pos: w.dictPos, SenseNo: 1, Ru: w.ru})
	}
	e.store.levelLexemes = map[string][]db.RandomLexemesByLevelRow{"A1": lex}
	return e, token
}

func TestNextMatchTopic(t *testing.T) {
	e, token := matchEnv(t)
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Kind != api.Match || ex.Topic != "skill:match" || ex.Prompt != "Соедини слова с переводами" || len(ex.Choices) != 0 {
		t.Fatalf("упражнение: %+v", ex)
	}
	if ex.Words == nil || ex.Options == nil || len(*ex.Words) != 5 || len(*ex.Options) != 5 {
		t.Fatalf("слова и варианты: %+v", ex)
	}
	if !slices.Equal(*ex.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("слова идут в порядке кандидатов: %v", *ex.Words)
	}
	for _, want := range []string{"яблоко", "дом", "вода", "книга", "собака"} {
		if !slices.Contains(*ex.Options, want) {
			t.Fatalf("нет перевода %q в %v", want, *ex.Options)
		}
	}
	var raw map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &raw)
	for _, k := range []string{"answer", "answers", "correct", "pairs"} {
		if _, ok := raw[k]; ok {
			t.Fatalf("в ответе есть поле %q: %s", k, rec.Body)
		}
	}
	stored := e.store.exercises[ex.Id]
	if stored.Kind != "match" || stored.SentenceID != nil || !slices.Contains(stored.Topics, "skill:match") {
		t.Fatalf("запись: %+v", stored)
	}
	a := storedAnswer(t, e, ex.Id)
	if len(a.Pairs) != 5 || a.Pairs[0] != (exercises.MatchPair{Word: "apple", Ru: "яблоко"}) ||
		len(a.Answers) != 1 || !strings.Contains(a.Answers[0], "apple=яблоко") {
		t.Fatalf("ответ в записи: %+v", a)
	}
}

func TestNextMatchPrefersWeakCards(t *testing.T) {
	e, token := matchEnv(t)
	e.store.weakWords = []db.ListWeakCardWordsRow{{LexemeID: 9, Lemma: "cat", Pos: "noun"}}
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Kind != api.Match || ex.Words == nil || (*ex.Words)[0] != "cat" {
		t.Fatalf("слабое слово карточек идёт первым: %+v", ex)
	}
}

func TestNextMatchSeparatesPartsOfSpeech(t *testing.T) {
	// у run есть и существительное, и глагол: переводы не должны смешиваться
	e, token := matchEnv(t, matchWords8...)
	e.store.translations = append(e.store.translations, db.ListTranslationsForWordsRow{Lemma: "run", Pos: "n", SenseNo: 1, Ru: "бег"})
	e.store.weakWords = []db.ListWeakCardWordsRow{{LexemeID: 7, Lemma: "run", Pos: "verb"}}
	ex := decodeExercise(t, getNext(t, e, token))
	a := storedAnswer(t, e, ex.Id)
	for _, p := range a.Pairs {
		if p.Word == "run" && p.Ru != "бежать" {
			t.Fatalf("глагол run получил перевод %q", p.Ru)
		}
	}
}

func TestNextMatchNeedsFiveWords(t *testing.T) {
	e, token := matchEnv(t, matchWords8[:4]...)
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, тело %s; с четырьмя словами вида быть не должно", rec.Code, rec.Body)
	}
}

func TestNextMatchWithoutDictionary(t *testing.T) {
	e, token := matchEnv(t)
	e.store.translations = nil
	if rec := getNext(t, e, token); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, тело %s; без словаря вида быть не должно", rec.Code, rec.Body)
	}
}

func TestNextMatchDictionaryErrorIsInternal(t *testing.T) {
	e, token := matchEnv(t)
	e.store.translations = nil
	e.store.failTranslations = true
	if rec := getNext(t, e, token); rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d; ошибка словаря — 500, а не молчаливый пропуск вида", rec.Code)
	}
}

// nextMatch выдаёт упражнение и возвращает его номер, верные пары и слова.
func nextMatch(t *testing.T, e env, token string) (int64, []exercises.MatchPair) {
	t.Helper()
	ex := decodeExercise(t, getNext(t, e, token))
	if ex.Kind != api.Match {
		t.Fatalf("ждали match: %+v", ex)
	}
	return ex.Id, storedAnswer(t, e, ex.Id).Pairs
}

func given(pairs []exercises.MatchPair) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.Word + "=" + p.Ru
	}
	return strings.Join(parts, ";")
}

func TestAnswerMatchAllCorrect(t *testing.T) {
	e, token := matchEnv(t)
	id, pairs := nextMatch(t, e, token)
	rec := e.do(http.MethodPost, answerPath, answerBody(id, given(pairs), "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	res := decodeResult(t, rec)
	if !res.Correct || res.Pairs == nil || len(*res.Pairs) != 5 || !strings.Contains(res.Answer, "apple=яблоко") {
		t.Fatalf("результат: %+v", res)
	}
	for _, p := range *res.Pairs {
		if !p.Correct {
			t.Fatalf("пара %+v", p)
		}
	}
	r, ok := ratingOf(res, "skill:match")
	if !ok || r.Rating <= 1500 || r.Attempts != 1 {
		t.Fatalf("рейтинг skill:match: %+v (есть %v)", r, ok)
	}
	if len(e.store.mistakes) != 0 {
		t.Fatalf("у сопоставления нет банка ошибок: %+v", e.store.mistakes)
	}
	evs := answerEvents(e)
	if len(evs) != 1 {
		t.Fatalf("событий %d", len(evs))
	}
	var p struct {
		Kind         string `json:"kind"`
		Correct      bool   `json:"correct"`
		CorrectPairs int    `json:"correctPairs"`
		Pairs        []struct {
			Word    string `json:"word"`
			Correct bool   `json:"correct"`
		} `json:"pairs"`
	}
	if err := json.Unmarshal(evs[0].Payload, &p); err != nil || p.Kind != "match" || !p.Correct || p.CorrectPairs != 5 || len(p.Pairs) != 5 {
		t.Fatalf("payload события: %s (%v)", evs[0].Payload, err)
	}
}

func TestAnswerMatchPartialGivesFractionalScore(t *testing.T) {
	e, token := matchEnv(t)
	id, pairs := nextMatch(t, e, token)
	swapped := slices.Clone(pairs)
	swapped[0].Ru, swapped[1].Ru = swapped[1].Ru, swapped[0].Ru // две пары перепутаны: 3 из 5
	rec := e.do(http.MethodPost, answerPath, answerBody(id, given(swapped), "k1"), withCookie(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	res := decodeResult(t, rec)
	if res.Correct || res.Pairs == nil {
		t.Fatalf("результат: %+v", res)
	}
	right := 0
	for i, p := range *res.Pairs {
		if p.Correct {
			right++
		}
		if p.Answer != pairs[i].Ru || p.Word != pairs[i].Word {
			t.Fatalf("пара %d: %+v, верно %+v", i, p, pairs[i])
		}
	}
	if right != 3 || (*res.Pairs)[0].Given != pairs[1].Ru {
		t.Fatalf("верных %d, пары %+v", right, *res.Pairs)
	}
	// рейтинг при 3 из 5 ниже, чем при 5 из 5
	e2, token2 := matchEnv(t)
	id2, pairs2 := nextMatch(t, e2, token2)
	full := decodeResult(t, e2.do(http.MethodPost, answerPath, answerBody(id2, given(pairs2), "k1"), withCookie(token2)))
	partial, _ := ratingOf(res, "skill:match")
	whole, _ := ratingOf(full, "skill:match")
	if !(partial.Rating < whole.Rating) {
		t.Fatalf("счёт дробный: 3 из 5 (%.1f) должно быть ниже 5 из 5 (%.1f)", partial.Rating, whole.Rating)
	}
	var ev struct {
		CorrectPairs int `json:"correctPairs"`
	}
	_ = json.Unmarshal(answerEvents(e)[0].Payload, &ev)
	if ev.CorrectPairs != 3 {
		t.Fatalf("correctPairs в событии: %d", ev.CorrectPairs)
	}
}

func TestAnswerMatchInvalidFormatIs400AndKeepsExercise(t *testing.T) {
	e, token := matchEnv(t)
	id, pairs := nextMatch(t, e, token)
	for name, g := range map[string]string{
		"мало пар":        given(pairs[:3]),
		"чужое слово":     strings.Replace(given(pairs), "apple=", "pear=", 1),
		"чужой перевод":   strings.Replace(given(pairs), "яблоко", "груша", 1),
		"повтор перевода": strings.Replace(given(pairs), "дом", "яблоко", 1),
		"без знака =":     "apple;house;water;book;dog",
	} {
		rec := e.do(http.MethodPost, answerPath, answerBody(id, g, "k-"+name), withCookie(token))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_answer") {
			t.Fatalf("%s: статус %d, тело %s", name, rec.Code, rec.Body)
		}
	}
	if e.store.exercises[id].AnsweredAt.Valid || len(answerEvents(e)) != 0 || len(e.store.topicRatings) > 6 && e.store.topicRatings[topicKey{1, "skill:match"}].Attempts != 0 {
		t.Fatalf("неверный формат не должен засчитываться")
	}
}

func TestAnswerMatchIdempotentAndConflict(t *testing.T) {
	e, token := matchEnv(t)
	id, pairs := nextMatch(t, e, token)
	body := answerBody(id, given(pairs), "k1")
	first := decodeResult(t, e.do(http.MethodPost, answerPath, body, withCookie(token)))
	again := e.do(http.MethodPost, answerPath, body, withCookie(token))
	if again.Code != http.StatusOK {
		t.Fatalf("повтор: статус %d, тело %s", again.Code, again.Body)
	}
	second := decodeResult(t, again)
	if !second.Correct || second.Pairs == nil || len(*second.Pairs) != 5 || second.Answer != first.Answer {
		t.Fatalf("повтор отдаёт тот же разбор: %+v", second)
	}
	r1, _ := ratingOf(first, "skill:match")
	r2, _ := ratingOf(second, "skill:match")
	if r1.Rating != r2.Rating || r2.Attempts != 1 || len(answerEvents(e)) != 1 {
		t.Fatalf("повтор по ключу не двигает рейтинг: %+v %+v, событий %d", r1, r2, len(answerEvents(e)))
	}
	other := e.do(http.MethodPost, answerPath, answerBody(id, given(pairs), "k2"), withCookie(token))
	if other.Code != http.StatusConflict {
		t.Fatalf("другой ключ: статус %d, ждали 409", other.Code)
	}
}

func TestAnswerMatchOtherUsersExerciseIs404(t *testing.T) {
	e, token := matchEnv(t)
	id, pairs := nextMatch(t, e, token)
	ex := e.store.exercises[id]
	ex.UserID = 2
	e.store.exercises[id] = ex
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, given(pairs), "k1"), withCookie(token)); rec.Code != http.StatusNotFound {
		t.Fatalf("статус %d, ждали 404", rec.Code)
	}
}

func TestAnswerGivenLimitIs400Characters(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	id := seedExercise(t, e, 1, "B1")
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, strings.Repeat("я", 401), "k"), withCookie(token)); rec.Code != http.StatusBadRequest {
		t.Fatalf("401 символ: статус %d, ждали 400", rec.Code)
	}
	if rec := e.do(http.MethodPost, answerPath, answerBody(id, strings.Repeat("я", 400), "k"), withCookie(token)); rec.Code != http.StatusOK {
		t.Fatalf("400 символов: статус %d, тело %s", rec.Code, rec.Body)
	}
}

// ruRanks кладёт русские слова в список частот фейка (ListWordFrequencies отдаёт только их); места по порядку, если не заданы.
func ruRanks(e env, ranks map[string]int32) {
	for w, r := range ranks {
		e.store.frequencies = append(e.store.frequencies, db.ListWordFrequenciesRow{Word: w, Rank: r})
	}
}

// withSenses заменяет переводы слова: каждое значение — отдельный номер sense_no.
func withSenses(e env, lemma, dictPos string, senses ...[]string) {
	e.store.translations = slices.DeleteFunc(e.store.translations, func(r db.ListTranslationsForWordsRow) bool { return r.Lemma == lemma })
	for i, sense := range senses {
		for _, ru := range sense {
			e.store.translations = append(e.store.translations, db.ListTranslationsForWordsRow{Lemma: lemma, Pos: dictPos, SenseNo: int16(i + 1), Ru: ru})
		}
	}
}

// otherRanks — места переводов остальных слов набора, чтобы они не отсеивались.
var otherRanks = map[string]int32{"дом": 100, "вода": 100, "книга": 100, "собака": 100, "кошка": 100, "бежать": 100, "большой": 100}

func TestMatchPrefersFrequentRussianVariant(t *testing.T) {
	e, token := matchEnv(t)
	withSenses(e, "apple", "n", []string{"батюшка", "папа"}, []string{"батюшка", "папа"})
	ruRanks(e, otherRanks)
	ruRanks(e, map[string]int32{"папа": 274, "батюшка": 30608})
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Options == nil || !slices.Contains(*ex.Options, "папа") || slices.Contains(*ex.Options, "батюшка") {
		t.Fatalf("ждали «папа» без «батюшки»: %+v", ex.Options)
	}
	if e.store.lastFreq == nil || e.store.lastFreq.SourceName != "frequencywords-ru" {
		t.Fatalf("места читаются из русского списка: %+v", e.store.lastFreq)
	}
}

func TestMatchWithoutRussianFrequenciesKeepsOldChoice(t *testing.T) {
	e, token := matchEnv(t) // список частот пуст: источник не загружен
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Words == nil || !slices.Equal(*ex.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("набор как до изменения: %+v", ex.Words)
	}
}

func TestMatchKeepsWordWithOnlyRareTranslation(t *testing.T) {
	e, token := matchEnv(t)
	withSenses(e, "apple", "n", []string{"батюшка"})
	ruRanks(e, otherRanks)
	ruRanks(e, map[string]int32{"батюшка": 30608})
	rec := getNext(t, e, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("статус %d, тело %s", rec.Code, rec.Body)
	}
	ex := decodeExercise(t, rec)
	if ex.Words == nil || !slices.Equal(*ex.Words, []string{"apple", "house", "water", "book", "dog"}) {
		t.Fatalf("слово с единственным редким переводом не теряется: %+v", ex.Words)
	}
}
