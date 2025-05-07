# HTTP Load Balancer (Go)

HTTP‑балансировщик с алгоритмом **round‑robin** и встроенным **Token‑Bucket rate‑limiter**.

---

## Возможности

* распределение запросов по пулу бэкендов (round‑robin);
* переключение на следующий сервер при ошибке;
* ограничение частоты запросов на IP (общий лимит + индивидуальные правила);
* логирование запросов, ошибок и превышений лимита;

---

## Docker Compose

1. Отредактируйте при необходимости `config.json`.
2. Поднимите всё одной командой:

```bash
docker compose up --build -d
```

Создаются четыре контейнера.
Проверка:

```bash
curl http://localhost:8080/
```

При последовательных вызовах ответы идут по кругу: `Backend 1 → 2 → 3 …`.

Остановить:

```bash
docker compose down
```

---

## Ручной запуск без Docker

```bash
go run ./cmd --config config.json
```

и отдельно поднять три любых HTTP‑сервера на 9001‑9003.

---

## Конфигурация (`config.json`)

```json
{
  "listen": ":8080",
  "backends": [
    "http://backend1:9001",
    "http://backend2:9002",
    "http://backend3:9003"
  ],
  "rate_limit": {
    "capacity": 10,
    "fill_rate": 5,
    "clients": [
      { "id": "127.0.0.1", "capacity": 4, "fill_rate": 2 }
    ]
  }
}
```

* **capacity** — максимальное число токенов в бакете.
* **fill_rate** — пополнение токенов за секунду.
* **clients** — индивидуальные лимиты (по IP или API‑ключу). Если значение 0, берётся общий default.

---

## Проверка лимитирования

```bash
# первые 10 запросов = 200, остальные = 429
for i in {1..15}; do \
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ ; \
done
```

Через 2 секунды токены пополнятся, следующие 5 запросов снова получат 200.

---

## Логи

```
[INFO ] GET / from 172.19.0.1 -> http://backend1:9001, 415µs
[WARN ] 429 GET / – IP 172.19.0.1
[ERROR] backend http://backend2:9002 недоступен: dial tcp ...
```
---

### Сборка образа вручную

```bash
docker build -t load-balancer .
docker run -p 8080:8080 load-balancer
```
