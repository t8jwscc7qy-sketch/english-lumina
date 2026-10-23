package auth

import (
	"sync"
	"time"
)

// sweepThreshold — размер таблицы, после которого при очередной попытке удаляются устаревшие записи.
const sweepThreshold = 1024

// Throttle ограничивает число попыток входа с одного ключа (IP). Состояние живёт в памяти:
// после перезапуска сервера счётчики обнуляются, для личного сервиса этого достаточно.
type Throttle struct {
	mu       sync.Mutex
	maxFails int
	block    time.Duration
	now      func() time.Time
	entries  map[string]*throttleEntry
}

type throttleEntry struct {
	attempts int
	last     time.Time // время последней попытки
	until    time.Time // до какого момента ключ заблокирован; нулевое значение — не заблокирован
}

func NewThrottle(maxFails int, block time.Duration, now func() time.Time) *Throttle {
	return &Throttle{maxFails: maxFails, block: block, now: now, entries: make(map[string]*throttleEntry)}
}

// Allow занимает попытку и сообщает, можно ли проверять пароль. Попытка засчитывается сразу,
// до медленной проверки: иначе параллельные запросы успели бы пройти все разом.
// maxFails попыток без Reset включают блокировку на block.
func (t *Throttle) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if len(t.entries) >= sweepThreshold {
		t.sweep(now)
	}

	e, ok := t.entries[key]
	if ok && !e.until.IsZero() && now.Before(e.until) {
		return false
	}
	if !ok || now.Sub(e.last) >= t.block {
		// Новая серия: записи нет, либо прошлые попытки давно в прошлом или блокировка истекла.
		e = &throttleEntry{}
		t.entries[key] = e
	}
	e.attempts++
	e.last = now
	if e.attempts >= t.maxFails {
		e.until = now.Add(t.block)
	}
	return true
}

// Reset забывает попытки ключа (после успешного входа).
func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// sweep удаляет записи, у которых и серия, и блокировка закончились.
func (t *Throttle) sweep(now time.Time) {
	for k, e := range t.entries {
		if now.Sub(e.last) >= t.block && !now.Before(e.until) {
			delete(t.entries, k)
		}
	}
}
