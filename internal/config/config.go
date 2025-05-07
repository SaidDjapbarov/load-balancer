// Этот пакет отвечает за загрузку и простую валидацию кофигурационного файла.
// Кофигурационный файл хранится в JSON.

package config

import (
	"encoding/json"
	"errors"
	"os"
)

// ClientLimit описывает индивидуальные лимиты для определенного клиента.
// Если capacity или fillrate будут равны нулю – будем использовать значения
// по умолчанию из общего блока rate_limit в config.json
type ClientLimit struct {
	ID       string `json:"id"`
	Capacity int    `json:"capacity"`
	FillRate int    `json:"fill_rate"`
}

// Config описывает поля из config.json.
// Поле Listen – это адрес, который будет слушать балансировщик.
// Backends – список адресов бэкенд-серверов.
type Config struct {
	Listen   string   `json:"listen"`   // если пустой, ставим ниже по умолчанию
	Backends []string `json:"backends"` // URLs бэкендов

	RateLimit struct {
		Capacity int           `json:"capacity"`  // сколько токенов максимум
		FillRate int           `json:"fill_rate"` // сколько токенов добавляется в секунду
		Clients  []ClientLimit `json:"clients"`   // индивидуальные клиенты
	} `json:"rate_limit"`
}

// Load читает и парсит конфиг.
// Если путь пустой, пытемся открыть "./config.json".
// Возвращаем готовую структуру *Config или ошибку.
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Общие лимиты, если явно они не указаны.
	if cfg.RateLimit.Capacity == 0 {
		cfg.RateLimit.Capacity = 10
	}
	if cfg.RateLimit.FillRate == 0 {
		cfg.RateLimit.FillRate = 5
	}

	// Простая валидация.
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}

	if len(cfg.Backends) == 0 {
		return nil, errors.New("в config.json должен быть хотя бы один backend")
	}

	return &cfg, nil
}
