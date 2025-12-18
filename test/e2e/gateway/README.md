# E2E (Gateway) — ms-go-user

Тесты проверяют флоу из `wiki/USER_PROFILE.md`, выполняя реальные запросы только через `ms-getway`.

## Требования
- Запущены контейнеры (gateway + user + auth + зависимости).
- Доступен student/guest gateway: `${GATEWAY_URL}` (по умолчанию `http://localhost:8080`).

## Переменные окружения
- `GATEWAY_URL` — base URL gateway.
- `HTTP_TIMEOUT` — таймаут curl (сек), по умолчанию `30`.
- `DEBUG=1` — подробный вывод.
- `MISMATCHES_OUT` — путь к файлу, куда дописывать найденные несоответствия (markdown).

## Запуск
```bash
cd ms-go-user
bash test/e2e/gateway/run-tests.sh
```
