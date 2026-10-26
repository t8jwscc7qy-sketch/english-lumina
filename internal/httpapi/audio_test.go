package httpapi

import (
	"net/http"
	"testing"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
)

func TestReadTextSentencesGetLiveAudioWithAllowedLicenseOnly(t *testing.T) {
	e, token := textsEnv(t, 25)
	ids := map[int64]bool{}
	// Записи для всех предложений: у чётных номеров разрешённая лицензия, у нечётных — NC-ND (не берётся).
	for w := 0; w < 6; w++ {
		for s := 0; s < 3; s++ {
			tid := int64(w*10+s+1) + 1000
			lic := "CC BY 4.0"
			if tid%2 == 1 {
				lic = "CC BY-NC-ND 3.0"
			}
			e.store.audio = append(e.store.audio, db.SentenceAudioForRow{SentenceTatoebaID: tid, AudioID: tid * 10, Username: "alice", License: lic})
			ids[tid] = lic == "CC BY 4.0"
		}
	}
	text, code, body := nextText(t, e, token, "")
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	withAudio := 0
	for _, s := range text.Sentences {
		allowed := ids[*s.TatoebaId]
		switch {
		case allowed && (s.Audio == nil || s.Audio.Url != "https://tatoeba.org/audio/download/"+itoa(*s.TatoebaId*10) || s.Audio.Author != "alice" || s.Audio.License != "CC BY 4.0"):
			t.Fatalf("у предложения %d должна быть озвучка: %+v", *s.TatoebaId, s.Audio)
		case !allowed && s.Audio != nil:
			t.Fatalf("озвучка с неразрешённой лицензией не отдаётся: %+v", s.Audio)
		}
		if s.Audio != nil {
			withAudio++
		}
	}
	if withAudio == 0 {
		t.Fatal("в тексте должна быть хотя бы одна озвученная фраза")
	}
	// Повторная выдача того же текста тоже с озвучкой.
	again, _, _ := nextText(t, e, token, "")
	n := 0
	for _, s := range again.Sentences {
		if s.Audio != nil {
			n++
		}
	}
	if again.Id != text.Id || n != withAudio {
		t.Fatalf("озвучка сохраняется при повторной выдаче: %d и %d", withAudio, n)
	}
}

func itoa(v int64) string {
	b := []byte{}
	if v == 0 {
		return "0"
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
