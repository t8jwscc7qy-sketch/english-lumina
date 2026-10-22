package auth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestThrottle(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	th := NewThrottle(5, time.Minute, clock.now)

	for i := 1; i <= 5; i++ {
		if !th.Allow("1.2.3.4") {
			t.Fatalf("попытка %d отклонена", i)
		}
	}
	if th.Allow("1.2.3.4") {
		t.Fatal("6-я попытка прошла")
	}
	if !th.Allow("5.6.7.8") {
		t.Fatal("блокировка задела другой ключ")
	}

	clock.t = clock.t.Add(time.Minute + time.Second)
	if !th.Allow("1.2.3.4") {
		t.Fatal("блокировка не снялась через минуту")
	}
}

func TestThrottleReset(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	th := NewThrottle(5, time.Minute, clock.now)

	for i := 0; i < 4; i++ {
		th.Allow("ip")
	}
	th.Reset("ip") // успешный вход
	for i := 1; i <= 5; i++ {
		if !th.Allow("ip") {
			t.Fatalf("после сброса попытка %d отклонена", i)
		}
	}
}

// Параллельные запросы не должны обходить лимит: попытка занимается до проверки пароля.
func TestThrottleConcurrent(t *testing.T) {
	th := NewThrottle(5, time.Minute, time.Now)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if th.Allow("ip") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 5 {
		t.Fatalf("прошло %d попыток из 100, ожидали ровно 5", got)
	}
}

// Записи с давно прошедшими попытками не копятся в памяти.
func TestThrottleSweepsStaleEntries(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	th := NewThrottle(5, time.Minute, clock.now)

	for i := 0; i < 3*sweepThreshold; i++ {
		th.Allow(fmt.Sprintf("ip-%d", i))
	}
	clock.t = clock.t.Add(2 * time.Minute)
	th.Allow("fresh")

	th.mu.Lock()
	n := len(th.entries)
	th.mu.Unlock()
	if n > 2 {
		t.Fatalf("в таблице %d записей, старые не удалены", n)
	}
}
