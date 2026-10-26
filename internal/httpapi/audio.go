package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/api"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/db"
	"github.com/t8jwscc7qy-sketch/english-lumina/internal/sources/tatoebaaudio"
)

// tatoebaAudioURL — адрес записи на tatoeba.org: файл не храним, браузер берёт его оттуда.
const tatoebaAudioURL = "https://tatoeba.org/audio/download/%d"

// Лицензии записей, которые сервер отдаёт; меняются при сборке сервера (WithNonCommercialAudio). По умолчанию открытые.
var servedAudioLicenses atomic.Pointer[[]string]

func audioLicenses() []string {
	if l := servedAudioLicenses.Load(); l != nil {
		return *l
	}
	return tatoebaaudio.OpenLicenses
}

// audioFor возвращает живые записи предложений Tatoeba по их номерам (разрешённые лицензии). Озвучка необязательна:
// ошибка чтения пишется в журнал, а ответ остаётся без звука (проигрыватель берёт синтез речи).
func audioFor(ctx context.Context, store AuthStore, tatoebaIDs []int64) map[int64]api.SentenceAudio {
	out := map[int64]api.SentenceAudio{}
	if len(tatoebaIDs) == 0 {
		return out
	}
	rows, err := store.SentenceAudioFor(ctx, db.SentenceAudioForParams{Ids: tatoebaIDs, Licenses: audioLicenses()})
	if err != nil {
		slog.Warn("озвучка недоступна", "err", err)
		return out
	}
	for _, r := range rows {
		a := api.SentenceAudio{Url: fmt.Sprintf(tatoebaAudioURL, r.AudioID), Author: r.Username, License: r.License}
		if r.AttributionUrl != nil && *r.AttributionUrl != "" {
			u := *r.AttributionUrl
			a.AttributionUrl = &u
		}
		out[r.SentenceTatoebaID] = a
	}
	return out
}

// attachExampleAudio добавляет озвучку английским предложениям примеров.
func attachExampleAudio(ctx context.Context, store AuthStore, examples []api.Example) {
	var ids []int64
	for _, e := range examples {
		ids = append(ids, e.En.TatoebaId)
	}
	m := audioFor(ctx, store, ids)
	for i := range examples {
		if a, ok := m[examples[i].En.TatoebaId]; ok {
			examples[i].En.Audio = &a
		}
	}
}

// attachReadAudio добавляет озвучку предложениям текста для чтения (у авторских предложений номера Tatoeba нет).
func attachReadAudio(ctx context.Context, store AuthStore, sentences []api.ReadSentence) {
	var ids []int64
	for _, s := range sentences {
		if s.TatoebaId != nil {
			ids = append(ids, *s.TatoebaId)
		}
	}
	m := audioFor(ctx, store, ids)
	for i := range sentences {
		if sentences[i].TatoebaId == nil {
			continue
		}
		if a, ok := m[*sentences[i].TatoebaId]; ok {
			sentences[i].Audio = &a
		}
	}
}
