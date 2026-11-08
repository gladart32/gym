# AUTH.md — план реализации аутентификации

`docs/AUTH.md` и `docs/DB.md` — справочная информация (схема БД, SQL-образцы, описание флоу).
Схема уже применена миграциями `backend/migrations/00001_users.sql` и `00002_auth.sql`.
**Этот файл — рабочий план привязки авторизации к коду Gym; при расхождении с docs приоритет у этого файла.**

Отклонения от docs/AUTH.md, принятые здесь: `email_verify` — 1 ч вместо рекомендованных 24 ч.

## Принятые решения

| Вопрос | Решение |
|---|---|
| JWT | `github.com/golang-jwt/jwt/v5`, HS256, секрет ≥ 32 байт |
| Хеш пароля | bcrypt (`golang.org/x/crypto/bcrypt`), стандартная стоимость |
| Access-токен | payload `{sub, sid, client_type, role, iat, exp}` — единый для web/mobile; TTL по средам (см. Конфигурацию) |
| Refresh-токен | 32 случайных байта (base64url, `crypto/rand`); в БД только SHA-256-хеш |
| Сессия | строка `login_sessions`, **скользящая**: `expires_at` продлевается при каждом refresh |
| Ротация | refresh одноразовый; старый хеш → `prev_refresh_token_hash`; предъявление prev = кража → отзыв сессии |
| Окно терпимости | `tolerance_sec` (10 с в dev/prod, 0 в test): prev, предъявленный сразу после ротации (две вкладки), не отзывает сессию — просто 401 |
| TTL токенов и писем | все сроки настраиваются по средам — см. Конфигурацию |
| Хранение на клиенте | **web**: refresh в HttpOnly-cookie, access — только в памяти (Pinia); **mobile**: оба токена в JSON → Keychain/Keystore. localStorage не используется |
| Данные юзера | `id, email, role` возвращаются в JSON ответов login/refresh — отдельный `GET /auth/me` не нужен |
| Подтверждение email | **строгое**: вход запрещён (403), пока `email_verified_at IS NULL`; токен жив 1 ч |
| Письма | asynq-таска `tasks:send_email`; пока SMTP нет — mailer-заглушка пишет ссылку в лог воркера |
| Отзыв сессий | смена пароля — все, кроме текущей; сброс по письму и блокировка админом — все |

Проверка каждого запроса — два шага (docs/AUTH.md, п.2): подпись и `exp` без базы,
затем один SELECT по `sid` (жива ли сессия). Это даёт мгновенный отзыв access-токена.

## Конфигурация

`internal/config` + `config/{dev,prod,test}.yaml`, по правилам backend/AGENT.md (поле → mapstructure →
дефолт → **обязательно ключ в `envKeys`**). Duration-поля парсит mapstructure, образец — `mysql.conn_max_lifetime`.

| Поле | dev | prod | test | env-переменная |
|---|---|---|---|---|
| `jwt_secret` | из yaml | только env, ≥ 32 байт | из yaml | `GYM_AUTH_JWT_SECRET` |
| `access_ttl` | 1h | 15m | 1m | `GYM_AUTH_ACCESS_TTL` |
| `refresh_ttl` | 720h | 720h | 1h | `GYM_AUTH_REFRESH_TTL` |
| `tolerance_sec` | 10 | 10 | 0 | `GYM_AUTH_TOLERANCE_SEC` |
| `email_verify_ttl` | 1h | 1h | 30s | `GYM_AUTH_EMAIL_VERIFY_TTL` |
| `password_reset_ttl` | 1h | 1h | 30s | `GYM_AUTH_PASSWORD_RESET_TTL` |

Дефолты (в `setDefaults`) — как в колонке prod. Запуск с пустым секретом в prod — fail-fast;
в dev/test допустим фиксированный секрет в yaml.

## Слои кода

- `internal/auth/` — сервис и репозиторий (sqlx, сырой SQL по образцу docs/AUTH.md):
  пароли, сессии (создание/ротация/отзыв/список), email-токены, выдача/разбор JWT
- `internal/transport/http/handler/auth.go` — эндпоинты
- `internal/transport/http/middleware/auth.go` — `Auth()` (два шага проверки, кладёт
  `user_id`, `sid`, `client_type`, `role` в контекст запроса) и `Admin()` (роль `admin`)
- Устройство и IP: mobile передаёт `device_name`/`os`/`app_version` явно, web — минимальный
  разбор `User-Agent`; IP из `c.ClientIP()` → 4/16 байт (аналог `INET6_ATON`)

## Эндпоинты (префикс /api/v1)

Публичные:

| Метод и путь | Поведение |
|---|---|
| `POST /auth/register` | создать пользователя + токен `email_verify` (`email_verify_ttl`) + письмо; ответ единообразен |
| `POST /auth/login` | bcrypt-проверка; отказ при `blocked_at` и неподтверждённом email (403); создать сессию, выдать пару токенов |
| `POST /auth/refresh` | ротация: новый refresh + access с тем же `sid`; prev-токен → отзыв сессии (кража), в окне `tolerance_sec` — просто 401; web читает refresh из cookie, mobile — из тела |
| `POST /auth/verify-email` | атомарное погашение токена (один UPDATE по `used_at`) → `email_verified_at` |
| `POST /auth/resend-verification` | повторное письмо с токеном `email_verify`, если пользователь есть и ещё не подтверждён |
| `POST /auth/forgot-password` | токен `password_reset` (`password_reset_ttl`); ответ не раскрывает наличие email |
| `POST /auth/reset-password` | атомарное погашение токена, новый пароль → отзыв **всех** сессий |

Защищённые (middleware `Auth()`):

| Метод и путь | Поведение |
|---|---|
| `POST /auth/logout` | отзыв своей сессии (`:session_id = :sid`), гашение refresh-cookie; клиент стирает токены |
| `GET /auth/sessions` | активные сессии с `is_current` (docs/AUTH.md, п.4) |
| `DELETE /auth/sessions/:id` | завершить свою сессию (проверка `user_id`) |
| `POST /auth/sessions/revoke-all` | завершить все, кроме текущей |
| `POST /auth/password` | смена пароля (нужен старый) → отзыв всех сессий, кроме текущей |

Формат ответов login/refresh: `{access_token, token_type: "Bearer", expires_in, user: {id, email, role}}`;
для mobile в JSON дополнительно `refresh_token`, для web refresh идёт только в Set-Cookie.
Конфликтов роутов в gin нет: `/sessions/revoke-all` (POST) и `/sessions/:id` (DELETE) — в разных деревьях методов.

## Фронт ↔ бек (web-клиент)

Запросы same-origin через nginx (prod) / vite-прокси (dev) — CORS не нужен, куки ходят без настроек.

**Почему без CORS**: браузер в dev и prod общается только с прокси (vite :5173 / nginx :80) —
один origin, соединение прокси→app серверное и CORS не касается. Если в будущем разведём
фронт и API на разные домены — понадобится `gin-contrib/cors` с конкретным Origin
+ `Access-Control-Allow-Credentials`, а refresh-кука — `SameSite=None; Secure` (обязателен HTTPS).
Осознанно отложено, как rate-limit и SMTP.

**Хранение**:
- `accessToken`, `tokenExpiresAt`, `user` — только в памяти (Pinia-стор `stores/auth.js`); localStorage не используется
- refresh-токен — HttpOnly-cookie, JS его не видит и не может украсть

**Кука `gym_refresh`**:
- атрибуты: `HttpOnly; SameSite=Strict; Path=/api/v1/auth/refresh; Max-Age=<refresh_ttl>`
  — Path скоупится только на refresh-эндпоинт, кука не летает с каждым запросом
- `Secure` — только в prod (dev идёт по http через vite-прокси)
- ставится в login/refresh, гасится в logout (`Max-Age=0`)
- на refresh сервер определяет client_type из сессии в БД — клиент его не передаёт

**Проактивный refresh (основной путь)**:
- ответ login/refresh содержит `expires_in`; стор считает `tokenExpiresAt = Date.now() + expires_in*1000`
- `REFRESH_MARGIN_MS = 10_000` — обновляем токен заранее, за 10 с до истечения
- request-интерцептор перед запросом с Bearer: `await ensureFreshToken()` → прикрепить токен;
  `ensureFreshToken()` — single-flight: параллельные запросы ждут один и тот же promise
- сам refresh-запрос идёт мимо этой логики (без Bearer) — рекурсии нет
- boot при перезагрузке страницы — тот же `ensureFreshToken()`: access в памяти нет → refresh по куке → юзер и токен в сторе

**Страховка на 401 (response-интерцептор, редкий путь)**:
- 401 на самом `/auth/refresh` → чистим стор, редирект на логин, без повторных попыток
- 401 на запросе с Bearer (глушитель часов, отзыв сессии) → single-flight refresh + **один** повтор
  с флагом «уже повторяли» — без зацикливания
- 401 без Bearer (неверный пароль на login и т.п.) не перехватывается

**Роутер**: boot-once в `router.beforeEach`; `meta.requiresAuth` → `/auth/login?redirect=...`;
`meta.guestOnly` (login/register) при живой сессии → `/`.

**Известное ограничение двух вкладок**: обе держат один access; после ротации второй refresh
получит 401 в окне терпимости (сессию не убьёт), но вкладка разлогинится. Осознанное поведение;
опциональное улучшение — синхронизация токена между вкладками через BroadcastChannel.

## Этапы

### Этап 1 — фундамент — ✅ готово
- Зависимости: `golang-jwt/jwt/v5`; `golang.org/x/crypto` из indirect в direct
- Секция конфига `auth:` (6 полей по таблице выше + `web_base_url` для ссылок в письмах) + `envKeys` + fail-fast на пустой секрет в prod
- Сервис JWT (выпуск/разбор access), middleware `Auth()` + `Admin()`
- CLI: `gym create-admin email password` (роль `admin`, `email_verified_at = now`)

Приёмка пройдена curl-ом на временной защите `/tasks/example` (снята после проверки).

### Этап 2 — ядро: register / login / refresh / logout — ✅ готово
- Регистрация с токеном подтверждения (`email_verify_ttl`) и постановкой письма в asynq
- Логин со всеми проверками, создание сессии (device/os/ip/UA), выдача токенов (web — cookie)
- Refresh: ротация одним UPDATE с защитой от гонки (docs/AUTH.md, п.3), prev-детекция кражи,
  скользящий `expires_at`, окно `tolerance_sec`
- Logout с гашением cookie
- Таска `tasks:send_email` с лог-mailer'ом (ссылка в лог воркера)

Приёмка: полный цикл curl-ом — register → ссылка из лога воркера → verify-email → login →
refresh → повтор со старым refresh → 401; prev после ротации вне окна → сессия отозвана.

### Этап 3 — сессии в личном кабинете — ✅ готово
- `GET /auth/sessions`, `DELETE /auth/sessions/:id`, `POST /auth/sessions/revoke-all`

Приёмка: после «Завершить» доступ убитого устройства падает сразу (access → 401).

### Этап 4 — пароль и email (критический путь входа) — ✅ готово
- Смена пароля; forgot/reset с атомарным погашением одноразовых токенов
- `verify-email` + `resend-verification`

Приёмка: сброс по ссылке гасит токен ровно один раз (второй клик — «ссылка недействительна»);
после сброса завершены все сессии, включая текущую.

### Этап 5 — фронтенд — ✅ готово
- `api/auth.js` — все auth-эндпоинты
- `stores/auth.js`: `accessToken`, `tokenExpiresAt`, `user`; действие `ensureFreshToken()` (single-flight)
- Интерцепторы `http.js`: проактивный refresh в request + страховка на 401 в response
- Guard'ы роутера: boot-once, `requiresAuth` / `guestOnly`
- Подключение готовых вью: Login/Register/ResetPassword/ConfirmEmail; список сессий в Settings

Приёмка: вход/выход из UI; после истечения access запросы продолжаются молча (refresh до запроса);
завершение сессии из ЛК выбрасывает устройство на экран входа.

### Этап 6 — тесты и укрепление — ✅ готово
Интеграционные (`internal/auth/auth_test.go`, testutil, реальная MySQL): ротация и гонка двух
параллельных refresh; кража по prev-токену → отзыв; окно терпимости (в test `tolerance_sec: 0`);
отзыв сессии бьёт access; вход без подтверждённого email → 403; одноразовость email-токенов и
истечение их TTL (проверяется сдвигом `expires_at` в прошлое, без sleep); смена/сброс пароля
отзывают нужные сессии; web-login ставит cookie и refresh по ней работает.
Отложенное: rate-limit на login/forgot/resend, SMTP-майлер, блокировка пользователя админом,
BroadcastChannel-синхронизация вкладок.

Дополнительно: `db:cleanup` теперь удаляет старые сессии и email-токены (п.8 docs/AUTH.md).

## Указатель по docs/AUTH.md

| Раздел | Что закрываем |
|---|---|
| п.1 Вход | этап 2 (login) |
| п.2 Проверка запроса | этап 1 (middleware) |
| п.3 Ротация и кража | этап 2 (refresh) |
| п.4–5 Сессии и отзыв | этапы 2–3 |
| п.6 Смена пароля / блокировка | этап 4 (блокировка админом — отложено) |
| п.7 Email-токены | этапы 2 и 4 (TTL — из конфига; отклонение от 24 ч см. в шапке) |
| п.8 Обслуживание | таска `db:cleanup` уже есть; дополнить удалением старых сессий |
