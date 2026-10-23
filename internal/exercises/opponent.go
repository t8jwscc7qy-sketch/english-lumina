package exercises

import (
	"strings"

	"github.com/t8jwscc7qy-sketch/english-lumina/internal/rating"
)

const (
	defaultOpponent = 1500.0 // сложность, если у упражнения нет ключа level:*
	opponentPhi     = 100.0
	opponentSigma   = 0.06
)

// levelDifficulty — рейтинг «соперника» по уровню предложения.
var levelDifficulty = map[string]float64{"A1": 800, "A2": 1000, "B1": 1200, "B2": 1400, "C1": 1600, "C2": 1800}

// Opponent берёт сложность упражнения из ключа level:* среди его тем; без него — 1500.
func Opponent(topics []string) rating.Rating {
	mu := defaultOpponent
	for _, t := range topics {
		if lvl, ok := strings.CutPrefix(t, "level:"); ok {
			if d, ok := levelDifficulty[lvl]; ok {
				mu = d
				break
			}
		}
	}
	return rating.Rating{Mu: mu, Phi: opponentPhi, Sigma: opponentSigma}
}
