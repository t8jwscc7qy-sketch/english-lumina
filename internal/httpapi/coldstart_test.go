package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

var coldWords = []string{"apple", "table", "river", "music", "chair", "cloud", "dream", "house", "light", "money", "night", "ocean",
	"paper", "queen", "radio", "smile", "train", "uncle", "voice", "water", "youth", "zebra", "bread", "candy", "dance", "earth",
	"field", "green", "happy", "juice", "knife", "lemon", "mouse", "noise", "olive", "plant", "quiet", "robot", "sugar", "tiger"}

// Сценарий «полный ноль»: нет ни одного слова в учёте. Читать нечего, «Знакомство» сразу даёт щедрый блок (холодный
// старт), и после первых блоков «Читать» собирает текст из показанных слов.
func TestColdStartFromZeroToFirstText(t *testing.T) {
	e := newExerciseEnv(t)
	token := login(t, e)
	e.store.settings = &db.Setting{UserID: 1, CurrentLevel: "A1", TargetLevel: "B1", DailyMinMinutes: 15, Timezone: "UTC"}
	e.store.kbBuilt = true
	poss := []string{"noun", "verb", "adjective", "adverb"}
	for i, w := range coldWords {
		addLearnWord(e, int64(i+1), w, poss[i%4], "A1", int32(10+i), true, "перевод-"+w)
	}
	// Целевые слова текстов и их предложения из показанных слов.
	blockExamples := e.store.examplesFor
	var rows []db.ListExamplesRow
	for w := 0; w < 6; w++ {
		target := fmt.Sprintf("tw%02d", w)
		payload, _ := json.Marshal(map[string]any{"lemma": target, "translations": []string{"перевод-" + target}})
		e.store.textTargets = append(e.store.textTargets, db.TextTargetWordsRow{LexemeID: int64(900 + w), Lemma: target, Pos: "noun", Payload: payload})
		e.store.addLexeme(int64(900+w), target, "noun", "A2")
		for s := 0; s < 3; s++ {
			id := int64(500 + w*10 + s)
			k := func(j int) string { return coldWords[(w*3+s*5+j)%21] }
			text := fmt.Sprintf("%s and %s %s with %s %s.", k(0), k(1), target, k(2), k(3))
			rows = append(rows, db.ListExamplesRow{EnID: id, EnTatoebaID: id + 1000, EnText: text, EnAuthor: "tom", RuTatoebaID: id + 2000, RuText: "перевод", RuAuthor: "ivan"})
		}
	}
	e.store.examplesFor = func(q string) []db.ListExamplesRow {
		var out []db.ListExamplesRow
		for _, r := range rows {
			for w := 0; w < 6; w++ {
				if strings.Contains(q, fmt.Sprintf("'tw%02d'", w)) && strings.Contains(r.EnText, fmt.Sprintf("tw%02d", w)) {
					out = append(out, r)
				}
			}
		}
		if len(out) > 0 {
			return out
		}
		return blockExamples(q)
	}

	// 1. Читать нечего.
	if _, code, body := nextText(t, e, token, ""); code != http.StatusNotFound || errorCode(t, body) != api.ErrorErrorCodeNoText {
		t.Fatalf("с нуля текста нет: %d %s", code, body)
	}
	// 2. Холодный старт: лимит 20, а не 5 по минутам.
	status := decodeStatus(t, getLearn(t, e, token, learnStatusPath).Body.String())
	if status.Limit != 20 || status.Reason.Code != api.LearnReasonCodeCold || status.Learned+status.Introduced != 0 {
		t.Fatalf("статус с нуля: %+v", status)
	}
	// 3. Блоки знакомства до порога слов для текста.
	introduced := 0
	for introduced < textMinKnown {
		rec := getLearn(t, e, token, learnBlockPath)
		if rec.Code != http.StatusOK {
			t.Fatalf("блок: %d %s", rec.Code, rec.Body)
		}
		block := decodeBlock(t, rec.Body.String())
		if len(block.Words) == 0 {
			t.Fatalf("блок пуст на %d словах: %+v", introduced, block)
		}
		ids := make([]int64, len(block.Words))
		for i, w := range block.Words {
			ids[i] = w.LexemeId
			if len(w.Examples) == 0 {
				t.Fatalf("слово %s без фразы", w.Lemma)
			}
		}
		if rec := introduce(e, token, ids, fmt.Sprintf("cold-%d", introduced)); rec.Code != http.StatusOK {
			t.Fatalf("introduce: %d %s", rec.Code, rec.Body)
		}
		introduced += len(ids)
	}
	if introduced > 28 {
		t.Fatalf("до порога понадобилось %d слов: холодный старт должен укладываться в несколько блоков", introduced)
	}
	// 4. Теперь читать можно: текст из показанных слов.
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatalf("после %d слов текст не собран: %d %s", introduced, code, body)
	}
	if text.Coverage < 0.75 || len(text.Sentences) < 6 || len(text.Targets) == 0 {
		t.Fatalf("первый текст: покрытие %v, предложений %d, целевых %d", text.Coverage, len(text.Sentences), len(text.Targets))
	}
}
