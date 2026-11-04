# AUTH.md — план реализации аутентификации

Источник истины по флоу и схеме — [`docs/AUTH.md`](docs/AUTH.md) (сессии, ротация refresh,
обнаружение кражи, SQL-запросы) и [`docs/DB.md`](docs/DB.md) (колонки таблиц).
Схема уже применена миграциями `backend/migrations/00001_users.sql` и `00002_auth.sql`.
Этот файл — дорожная карта привязки авторизации к коду Gym; при расхождении приоритет у docs/AUTH.md.

## Принятые решения

| Вопрос | Решение |
|---|---|
| JWT | `github.com/golang-jwt/jwt/v5`, HS256, секрет ≥ 32 байт |
| Хеш пароля | bcrypt (`golang.org/x/crypto/bcrypt`), стандартная стоимость |
| Access-токен | 15 минут; payload `{sub, sid, client_type, role, iat, exp}` — единый для web/mobile |
| Refresh-токен | 32 случайных байта (base64url, `crypto/rand`); в БД только SHA-256-хеш |
| Сессия | строка `login_sessions`, **скользящая**: `expires_at` продлевается при каждом refresh |
| Ротация | refresh одноразовый; старый хеш → `prev_refresh_token_hash`; предъявление prev = кража → отзыв сессии |
| Окно терпимости | 10 секунд: prev, предъявлённый сразу после ротации (две вкладки), не отзывает сессию — просто 401 |
| Хранение на клиенте | **web**: refresh в cookie `HttpOnly; Secure; SameSite=Strict`, access — в памяти страницы; **mobile**: оба токена в Keychain/Keystore (пока web-only, mobile — JSON-ответ) |
| Подтверждение email | **строгое**: вход запрещён (403), пока `email_verified_at IS NULL` |
| Письма | asynq-таска `tasks:send_email`; пока SMTP нет — mailer-заглушка пишет ссылку в лог воркера |
| Отзыв сессий | смена пароля — все, кроме текущей; сброс по письму и блокировка админом — все |

Проверка каждого запроса — два шага (docs/AUTH.md, п.2): подпись и `exp` без базы,
затем один SELECT по `sid` (жива ли сессия). Это даёт мгновенный отзыв access-токена.

## Конфигурация

`internal/config` + `config/{dev,prod,test}.yaml`, по правилам backend/AGENT.md (поле → mapstructure →
дефолт → **обязательно ключ в `envKeys`**):

```yaml
auth:
  jwt_secret: ""            # GYM_AUTH_JWT_SECRET; в prod — только env, минимум 32 байт
  access_ttl: "15m"         # GYM_AUTH_ACCESS_TTL
  refresh_ttl: "720h"       # GYM_AUTH_REFRESH_TTL (30 дней)
  tolerance_sec: 10         # GYM_AUTH_TOLERANCE_SEC — окно терпимости ротации
```

В dev/test допустим фиксированный секрет в yaml; запуск с пустым секретом в prod — fail-fast.

## Слои кода

- `internal/auth/` — сервис и репозиторий (sqlx, сырой SQL по образцу docs/AUTH.md):
  пароли, сессии (создание/ротация/отзыв/список), email-токены, выдача/разбор JWT
- `internal/transport/http/handler/auth.go` — эндпоинты
- `internal/transport/http/middleware/auth.go` — `Auth()` (два шага проверки, кладёт
  `user_id`, `sid`, `client_type`, `role` в контекст запроса) и `Admin()` (роль `admin`)
- Устройство и IP: mobile передаёт `device_name`/`os`/`app_version` явно, web — минимальный
  разбор `User-Agent`; IP из `c.ClientIP()` → 4/16 байт (аналог `INET6_ATON`)

## Эндпоинты (префикс /api/v1)

| Метод и путь | Поведение |
|---|---|
| `POST /auth/register` | создать пользователя + токен `email_verify` (24 ч) + письмо; ответ единообразен |
| `POST /auth/login` | bcrypt-проверка; отказ при `blocked_at` и неподтверждённом email (403); создать сессию, выдать пару токенов |
| `POST /auth/refresh` | ротация: новый refresh + access с тем же `sid`; prev-токен → отзыв сессии (кража), в окне 10 с — просто 401; web читает refresh из cookie |
| `POST /auth/logout` | отзыв своей сессии (`:session_id = :sid`), клиент стирает токены |
| `GET /auth/sessions` | активные сессии с `is_current` (docs/AUTH.md, п.4) |
| `DELETE /auth/sessions/:id` | завершить свою сессию (проверка `user_id`) |
| `POST /auth/sessions/revoke-all` | завершить все, кроме текущей |
| `POST /auth/password` | смена пароля (нужен старый) → отзыв всех сессий, кроме текущей |
| `POST /auth/forgot-password` | токен `password_reset` (1 ч); ответ не раскрывает наличие email |
| `POST /auth/reset-password` | атомарное погашение токена (один UPDATE по `used_at`), новый пароль → отзыв **всех** сессий |
| `POST /auth/verify-email` | атомарное погашение токена → `email_verified_at` |

Клиентские детали: ответ login/refresh для web — refresh в cookie, access в JSON; для mobile — оба в JSON.
Web-запросы на `/auth/refresh` идут same-origin через nginx/vite-прокси, отдельный CORS не нужен.

## Этапы

### Этап 1 — фундамент
- Зависимости: `golang-jwt/jwt/v5`; `golang.org/x/crypto` из indirect в direct
- Секция конфига `auth:` + `envKeys` + fail-fast на пустой секрет в prod
- Сервис JWT (выпуск/разбор access), middleware `Auth()` + `Admin()`
- CLI: `gym create-admin email password` (роль `admin`, `email_verified_at = now`)

Приёмка: защищённый тест-эндпоинт без токена → 401, с валидным → 200; токен отозванной сессии → 401.

### Этап 2 — ядро: register / login / refresh / logout
- Регистрация с токеном подтверждения и постановкой письма в asynq
- Логин со всеми проверками, создание сессии (device/os/ip/UA), выдача токенов
- Refresh: ротация одним UPDATE с защитой от гонки (docs/AUTH.md, п.3),
  prev-детекция кражи, скользящий `expires_at`
- Logout

Приёмка: полный цикл curl-ом — register → ссылка из лога воркера → verify-email → login →
refresh → повтор со старым refresh → 401; prev после ротации вне окна → сессия отозвана.

### Этап 3 — сессии в личном кабинете
- `GET /auth/sessions`, `DELETE /auth/sessions/:id`, `POST /auth/sessions/revoke-all`

Приёмка: после «Завершить» доступ убитого устройства падает сразу (access → 401).

### Этап 4 — пароль и email (критический путь входа)
- Смена пароля; forgot/reset с атомарным погашением одноразовых токенов
- `verify-email` + повторная отправка письма; таска `tasks:send_email` с лог-mailer'ом

Приёмка: сброс по ссылке гасит токен ровно один раз (второй клик — «ссылка недействительна»);
после сброса завершены все сессии, включая текущую.

### Этап 5 — фронтенд
- axios-интерцепторы: access из памяти, на 401 — единая очередь refresh и повтор запроса
- Pinia auth-store, guards роутера (незалогинен → `/auth/login`)
- Подключение готовых вью: Login/Register/ResetPassword/ConfirmEmail; список сессий в Settings

Приёмка: вход/выход из UI; завершение сессии из ЛК выбрасывает устройство на экран входа.

### Этап 6 — тесты и укрепление
Интеграционные (testutil, реальная MySQL): ротация и гонка двух параллельных refresh;
кража по prev-токену → отзыв; отзыв сессии бьёт access; вход без подтверждённого email → 403;
одноразовость email-токенов; смена/сброс пароля отзывают нужные сессии.
Отложенное: rate-limit на login/forgot, SMTP-майлер, блокировка пользователя админом.

## Указатель по docs/AUTH.md

| Раздел | Что закрываем |
|---|---|
| п.1 Вход | этап 2 (login) |
| п.2 Проверка запроса | этап 1 (middleware) |
| п.3 Ротация и кража | этап 2 (refresh) |
| п.4–5 Сессии и отзыв | этапы 2–3 |
| п.6 Смена пароля / блокировка | этап 4 |
| п.7 Email-токены | этапы 2 и 4 |
| п.8 Обслуживание | таска `db:cleanup` уже есть; дополнить удалением старых сессий |
