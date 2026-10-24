package exercises

import (
	"math/rand/v2"
	"slices"
)

// Reshuffle возвращает новый срез с теми же элементами в другом случайном порядке (по seed).
// forbidden может быть nil; если она вернула true, перемешиваем заново — до shuffleAttempts раз.
// false, если избежать запрещённого порядка не удалось.
func Reshuffle(items []string, forbidden func([]string) bool, seed int64) ([]string, bool) {
	out := slices.Clone(items)
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
	for range shuffleAttempts {
		rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		if forbidden == nil || !forbidden(out) {
			return out, true
		}
	}
	return nil, false
}
