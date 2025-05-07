// Этот пакет содержит одну простую реализацию балансировки – Round-Robin.
package balancer

import "sync/atomic"

// RoundRobin хранит срез адресов бэкендов и счетчик.
// Счетчик увеличивается атомарно – для гарантии, что несколько горутин не выдадут
// один и тот же бэкенд одновременно.
type RoundRobin struct {
	backends []string
	counter  uint64
}

// Создает балансировщик.
func NewRoundRobin(backends []string) *RoundRobin {
	return &RoundRobin{
		backends: backends,
	}
}

// Возвращает адрес следующего бэкенда.
func (rr *RoundRobin) Next() string {
	// atomic.AddUint64 возвращает новое значение счетчика.
	// т.к. индекс начинается с 0, то вычитаем 1.
	idx := atomic.AddUint64(&rr.counter, 1)
	n := uint64(len(rr.backends))
	return rr.backends[(idx-1)%n]
}
