// Запускает балансировщик и токен-бакет-лимитер.
//
// Как работает:
//  1. Для начала читаем конфиг.
//  2. Создаем объекты Round-Robin и Limiter.
//  3. Запускаем http.Server с Handler.
//     Handler сначала проверяет лимит, потом пытается найти живой бэкенд.
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"load-balancer/internal/balancer"
	"load-balancer/internal/config"
	"load-balancer/internal/ratelimiter"
)

// Проверяет, закрыт ли канал.
func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func main() {
	// --- 1. Кофиг ---
	configPath := flag.String("config", "config.json", "путь к конфигурационному файлу")
	flag.Parse()

	// Загружаем конфиг.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("не удалось загрузить конфиг: %v", err)
	}

	log.Printf("запускаем балансировщик. порт: %s, бэкенды: %v", cfg.Listen, cfg.Backends)

	// --- 2. Балансировщик и лимитер ---
	rr := balancer.NewRoundRobin(cfg.Backends)

	// Индвидуальные лимиты через map (IP -> LimitCfg)
	indiv := make(map[string]ratelimiter.LimitCfg)
	for _, c := range cfg.RateLimit.Clients {
		if c.Capacity == 0 {
			c.Capacity = cfg.RateLimit.Capacity
		}
		if c.FillRate == 0 {
			c.FillRate = cfg.RateLimit.FillRate
		}
		indiv[c.ID] = ratelimiter.LimitCfg{Capacity: c.Capacity, FillRate: c.FillRate}
	}

	limiter := ratelimiter.NewLimiter(
		cfg.RateLimit.Capacity,
		cfg.RateLimit.FillRate,
		indiv,
	)
	defer limiter.Stop()

	// --- 3. HTTP-хэндлер ---
	handler := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)

		// Проверяем лимит.
		if !limiter.Allow(clientIP) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			log.Printf("[WARN] 429 %s %s — IP %s", r.Method, r.URL.Path, clientIP)
			return
		}

		// Обходим все бекенды по кругу – ищем живой.
		for i := 0; i < len(cfg.Backends); i++ {
			backendAddr := rr.Next()
			targetURL, _ := url.Parse(backendAddr)
			if err != nil {
				// Это баг конфигурации, поэтому просто логируем и пробуем следующий.
				log.Printf("[ERROR] не парсится URL %s: %v", backendAddr, err)
				continue
			}

			done := make(chan struct{}) // закроем при ошибке прокси
			proxy := httputil.NewSingleHostReverseProxy(targetURL)

			proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
				log.Printf("[ERROR] бекенд %s недоступен: %v", backendAddr, err)
				close(done) // сигнал "пытаемся дальше"
			}

			// Пробуем отдать запрос.
			proxy.ServeHTTP(w, r)

			// Если прокси вернул ошибку — идём на следующий бекенд.
			if isClosed(done) {
				continue
			}

			// Успех, нашли.
			log.Printf("[INFO] %s %s IP %s -> %s, %v", r.Method, r.URL.RequestURI(), clientIP, backendAddr, time.Since(start))
			return
		}

		// Сюда дошли — значит, все бэкенды упали.
		http.Error(w, "all backends down", http.StatusBadGateway)
	}

	server := &http.Server{
		Addr:         cfg.Listen,
		Handler:      http.HandlerFunc(handler),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("ошибка сервера: %v", err)
	}
}
