// Этот пакет реализовывает модель Token-Bucket.
// Каждый клиент получает свой bucket.
package ratelimiter

import (
	"sync"
	"sync/atomic"
	"time"
)

// bucket – внутренний счетчик токенов.
// mutex защищает поля backet.
type bucket struct {
	mu       sync.Mutex
	tokens   int
	capacity int
	fillRate int // сколько токенов добавляется каждую секунду
}

// Возврщает true, если удалось выдать токен.
func (b *bucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.tokens == 0 {
		return false
	}
	b.tokens--
	return true
}

// Пополняет токены, при этом не превышая емкости.
func (b *bucket) refill() {
	b.mu.Lock()
	b.tokens += b.fillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.mu.Unlock()
}

// ------- Limiter --------

// LimitCfg задает лимит для определенного клиента.
// Если явно не указан лимит, то применяем дефолтный.
type LimitCfg struct {
	Capacity int
	FillRate int
}

// Limiter хранит все бакеты и их настройки (общие и индивидуальные).
// Для остановки фоновой горутины refillLoop используется atomic.Bool.
type Limiter struct {
	buckets sync.Map            // key -> *bucket
	configs map[string]LimitCfg // индивидуальные лимиты
	defCfg  LimitCfg            // дефолтный лимит
	stopped atomic.Bool         // флаг для завершения refillLoop
}

// NewLimiter создает Limiter с дефолтными и индивидуальными лимитами.
// Запускаем фоновую горутину для автозаполнения токенов.
func NewLimiter(defaultCap, defaultFill int, indiv map[string]LimitCfg) *Limiter {
	l := &Limiter{
		configs: indiv,
		defCfg:  LimitCfg{defaultCap, defaultFill},
	}
	go l.refillLoop()
	return l
}

// Allow проверяет, можно ли выполнить запрос для ключа key.
// Если backet еще не создан, создаем его.
func (l *Limiter) Allow(key string) bool {
	val, ok := l.buckets.Load(key)
	if !ok {
		// подбираем лимит для клиента
		cfg, found := l.configs[key]
		if !found {
			cfg = l.defCfg
		}
		val, _ = l.buckets.LoadOrStore(key, &bucket{
			tokens:   cfg.Capacity,
			capacity: cfg.Capacity,
			fillRate: cfg.FillRate,
		})
	}
	return val.(*bucket).take()
}

// Останавливает refillLoop.
func (l *Limiter) Stop() { l.stopped.Store(true) }

// Работает на фоне и раз в секунду проходит по map и пополняет токены, вызывая refill.
func (l *Limiter) refillLoop() {
	ticker := time.NewTicker(time.Second)
	for !l.stopped.Load() {
		<-ticker.C
		l.buckets.Range(func(_, v any) bool {
			v.(*bucket).refill()
			return true
		})
	}
	ticker.Stop()
}
