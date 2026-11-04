# Аутентификация и сессии

Здесь описано, как работают вход, JWT-токены и сессии пользователя на таблицах `users`, `login_sessions` и `email_action_tokens`. Колонки таблиц описаны в [DB.md](DB.md).

## Главная идея

**В базе хранится сессия, а не токены.**

- Сессия — это одна строка в `login_sessions`, одна на каждый вход с устройства (браузер, телефон, планшет).
- Access-токен в базе не хранится. Он проверяется по подписи, а к сессии привязан через поле `sid` в payload.
- Refresh-токен хранится только как SHA-256-хеш (`refresh_token_hash`, 32 байта, какой бы длины ни был сам токен). Хеш предыдущего токена лежит в `prev_refresh_token_hash`: по нему обнаруживается кража ([п.3](#зачем-prev_refresh_token_hash-обнаружение-кражи-refresh-токена)).
- Отозвать сессию — значит заполнить `revoked_at`. После этого и access, и refresh этой сессии перестают приниматься **сразу**, потому что сервер проверяет сессию на каждый запрос.

```
              ┌─────────────── login_sessions ────────────────┐
  телефон ──► │ id=17  mobile  iPhone 15 / iOS 18.1   активна │
  браузер ──► │ id=18  web     Chrome / Windows 11    активна │ ◄── user_id=5
  планшет ──► │ id=21  mobile  Galaxy Tab / Android   отозвана│
              └───────────────────────────────────────────────┘
  access JWT телефона: { sub: 5, sid: 17, client_type: "mobile", exp: … }
```

## Токены

| | Access | Refresh |
|---|---|---|
| Формат | JWT, подписанный (HS256 / EdDSA) | случайные 32+ байта (base64url) или JWT |
| Срок жизни (рекомендуемый) | 15 минут | 30 дней (`login_sessions.expires_at`) |
| Где проверяется срок | в самом токене (`exp`) | в базе (`expires_at`) |
| Хранится в базе | **нет** | только `SHA-256(refresh)`: текущего и предыдущего |
| Для чего | доступ к API | получить новую пару токенов |

Payload access-токена одинаков для web и mobile. Флоу различаются только значением `client_type`:
```json
{
  "sub": 5,                // users.id
  "sid": 17,               // login_sessions.id
  "client_type": "mobile", // "web" | "mobile"
  "role": "user",
  "iat": 1762170000,
  "exp": 1762170900
}
```

Как клиенту хранить токены (рекомендация):
- **web**: refresh в cookie `HttpOnly; Secure; SameSite=Strict`, чтобы JS не мог его прочитать; access в памяти страницы;
- **mobile**: оба токена в защищённом хранилище ОС (Keychain / Android Keystore).

---

## 1. Вход

Клиент присылает email, пароль и данные устройства. Мобильное приложение передаёт `device_name`, `os` и `app_version` явно, для web сервер определяет их из `User-Agent`.

```
POST /auth/login
{ "email": "...", "password": "...", "client_type": "mobile",
  "device_name": "iPhone 15", "os": "iOS 18.1", "app_version": "1.2.0" }
```

Сервер:
1. находит пользователя и сверяет пароль с `users.password_hash`;
2. проверяет `blocked_at IS NULL` (и `email_verified_at IS NOT NULL`, если без подтверждения входить нельзя);
3. генерирует refresh-токен и создаёт сессию:
   ```sql
   INSERT INTO login_sessions
       (user_id, client_type, refresh_token_hash, device_name, os, app_version, user_agent, ip, expires_at)
   VALUES
       (5, 'mobile', UNHEX(SHA2(:refresh, 256)), 'iPhone 15', 'iOS 18.1', '1.2.0',
        :user_agent, INET6_ATON(:ip), UTC_TIMESTAMP() + INTERVAL 30 DAY);
   ```
   `UNHEX(SHA2(…))` здесь только для наглядности. В приложении хеш лучше считать в коде (`sha256.Sum256`) и передавать готовые 32 байта, чтобы сам токен не попадал в логи запросов MySQL;
4. выпускает access JWT с `sid` = id новой сессии и возвращает оба токена.

---

## 2. Проверка каждого запроса

```
GET /api/workouts
Authorization: Bearer <access>
```

1. **Без базы:** проверить подпись и `exp`. Если не прошло, вернуть 401.
2. **С базой:** проверить, что сессия из `sid` жива:
   ```sql
   SELECT 1 FROM login_sessions
   WHERE id = :sid AND user_id = :sub
     AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP();
   ```
   Если строки нет, вернуть 401, хотя подпись и `exp` у токена в порядке.

Шаг 2 — это один запрос по первичному ключу. Он и даёт мгновенный отзыв. Если нагрузка вырастет, результат можно кешировать в памяти или Redis на 10–30 секунд: тогда отзыв срабатывает с такой же задержкой.

> **Альтернатива без шага 2.** Не проверять сессию, а сделать access короче (5 минут). Отозванное устройство тогда работает, пока не истечёт его access, а потом не сможет обновиться. Для ЛК с кнопкой «завершить сессию» это хуже: пользователь ждёт, что устройство отключится сразу.

---

## 3. Обновление токенов (refresh)

```
POST /auth/refresh
{ "refresh_token": "..." }      // web: из HttpOnly cookie
```

Каждый refresh-токен одноразовый. При обновлении клиент получает новый, а старый перестаёт действовать. Это называется **ротация**.

Сервер:
1. ищет активную сессию по хешу **текущего** токена:
   ```sql
   SELECT id, user_id, client_type FROM login_sessions
   WHERE refresh_token_hash = :hash
     AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP();
   ```
2. **нашёл**: ротирует токен. Выдаёт новый, а текущий хеш переносит в `prev_refresh_token_hash`:
   ```sql
   UPDATE login_sessions
   SET prev_refresh_token_hash = refresh_token_hash,   -- старый токен запоминаем
       refresh_token_hash      = :new_hash,             -- новый становится текущим
       last_used_at            = UTC_TIMESTAMP()
   WHERE id = :sid AND refresh_token_hash = :old_hash;  -- защита от гонки двух refresh
   ```
   MySQL выполняет присваивания в `SET` слева направо, поэтому в `prev_refresh_token_hash` попадает старое значение, ещё до перезаписи. Если затронуто 0 строк, токен успели использовать параллельно: вернуть 401. Иначе выпустить новый access с тем же `sid`;
3. **не нашёл**: проверяет, не предъявлен ли **предыдущий**, уже заменённый токен (см. ниже), и возвращает 401.

`id` сессии при ротации не меняется. Устройство остаётся той же строкой в списке ЛК, обновляется только `last_used_at`.

Срок сессии можно продлевать при каждом refresh (`expires_at = UTC_TIMESTAMP() + INTERVAL 30 DAY`), тогда сессия «скользящая» и истекает только после 30 дней неактивности. Можно и не продлевать: тогда раз в 30 дней нужно входить заново.

### Зачем `prev_refresh_token_hash`: обнаружение кражи refresh-токена

> **Коротко.** Настоящий клиент никогда не предъявляет старый refresh-токен: после ротации он хранит только новый. Если сервер получил **предыдущий** токен, значит, его копия есть у кого-то ещё. Сервер не может понять, кто из двоих хозяин, поэтому **отзывает сессию целиком**, и злоумышленник теряет доступ.

Без этой колонки кражу не обнаружить. Сравним два сценария: злоумышленник украл refresh `R1` (вредоносное расширение, перехват, доступ к телефону) и **воспользовался им первым**.

**Без `prev_refresh_token_hash`:**
```
злоумышленник: refresh R1  →  получает R2 (в БД теперь R2)  →  пользуется API
хозяин:        refresh R1  →  R1 в БД не найден  →  401  →  экран входа
               хозяин входит заново: новая сессия
итог: сессия злоумышленника с R2 ОСТАЁТСЯ АКТИВНОЙ, кража не замечена
```

**С `prev_refresh_token_hash`:**
```
злоумышленник: refresh R1  →  получает R2 (refresh_token_hash = R2, prev_refresh_token_hash = R1)
хозяин:        refresh R1  →  R1 не текущий, но совпал с prev  →  КРАЖА
               →  сессия отзывается (revoked_at)  →  401
злоумышленник: запрос с access или refresh R2  →  сессия отозвана  →  401
итог: доступ злоумышленника закрыт сразу, как только хозяин обратился к серверу
```

Если первым токеном воспользовался хозяин, а злоумышленник пришёл позже со старым `R1`, это та же ситуация «предъявлен prev», и сессия тоже отзывается.

Проверка в шаге 3:
```sql
-- предъявлен предыдущий токен этой сессии?
SELECT id, user_id, last_used_at FROM login_sessions
WHERE prev_refresh_token_hash = :hash AND revoked_at IS NULL;

-- если нашли: отозвать сессию
UPDATE login_sessions SET revoked_at = UTC_TIMESTAMP() WHERE id = :sid;
```

Дополнительно стоит:
- записать событие в лог безопасности (user_id, sid, ip);
- показать пользователю после входа предупреждение: «Сессия на iPhone 15 была завершена из-за подозрительной активности»;
- по желанию, отозвать **все** сессии пользователя и предложить сменить пароль.

**Ложные срабатывания.** Если у настоящего клиента два запроса refresh ушли почти одновременно (две вкладки браузера, повтор после обрыва сети), второй придёт со старым токеном, хотя кражи нет. Чтобы не выкидывать пользователя зря, задайте **окно терпимости**: если предыдущий токен предъявлен в течение нескольких секунд после ротации (`last_used_at > UTC_TIMESTAMP() - INTERVAL 10 SECOND`), вернуть 401 без отзыва сессии, клиент просто повторит запрос с актуальным токеном. Надёжнее, чтобы клиент сам не отправлял refresh параллельно (одна общая операция обновления на всё приложение).

---

## 4. Активные сессии в личном кабинете

`:current_sid` — это `sid` из access-токена того, кто открыл страницу.

```sql
SELECT id, client_type, device_name, os, app_version,
       INET6_NTOA(ip) AS ip, created_at, last_used_at,
       id = :current_sid AS is_current
FROM login_sessions
WHERE user_id = :user_id
  AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP()
ORDER BY is_current DESC, last_used_at DESC;
```

Пример ответа:

| id | client_type | device_name | os | app_version | last_used_at | is_current |
|---|---|---|---|---|---|---|
| 18 | web | Chrome | Windows 11 | — | только что | 1 |
| 17 | mobile | iPhone 15 | iOS 18.1 | 1.2.0 | 2 часа назад | 0 |
| 23 | mobile | Galaxy Tab S9 | Android 15 | 1.2.0 | вчера | 0 |
| 25 | web | Safari | macOS 15 | — | 5 дней назад | 0 |

В интерфейсе это выглядит так:
```
Активные сессии
  ● Chrome · Windows 11                 это устройство
  ○ iPhone 15 · iOS 18.1 · v1.2.0       2 часа назад      [Завершить]
  ○ Galaxy Tab S9 · Android 15          вчера             [Завершить]
  ○ Safari · macOS 15                   5 дней назад      [Завершить]
                                          [Завершить все, кроме этой]
```

---

## 5. Завершение сессий

Во всех запросах есть условие `user_id = :user_id`: пользователь может завершить только свои сессии, даже если подставит чужой `id`.

**Завершить одну сессию** (кнопка «Завершить» в списке):
```sql
UPDATE login_sessions SET revoked_at = UTC_TIMESTAMP()
WHERE id = :session_id AND user_id = :user_id AND revoked_at IS NULL;
```

**Завершить все, кроме текущей:**
```sql
UPDATE login_sessions SET revoked_at = UTC_TIMESTAMP()
WHERE user_id = :user_id AND id <> :current_sid AND revoked_at IS NULL;
```

**Выйти на текущем устройстве (logout):** то же, что «завершить одну», с `:session_id = :current_sid`. Клиент удаляет у себя токены.

**Выйти везде, включая текущее:**
```sql
UPDATE login_sessions SET revoked_at = UTC_TIMESTAMP()
WHERE user_id = :user_id AND revoked_at IS NULL;
```

Что происходит после отзыва, на примере телефона (сессия 17):
```
ЛК (браузер, sid=18):  «Завершить» у iPhone  →  UPDATE … SET revoked_at WHERE id=17
телефон:  GET /api/…  Bearer <access sid=17>
          1. подпись и exp в порядке
          2. SELECT … WHERE id=17 AND revoked_at IS NULL  → пусто  → 401
          POST /auth/refresh                              → хеш не найден среди активных → 401
          → экран входа
```

---

## 6. Смена пароля и блокировка

| Событие | Что делать с сессиями |
|---|---|
| Пользователь сменил пароль в ЛК (знает старый) | завершить все, **кроме текущей** |
| Пароль сброшен по ссылке из письма | завершить **все** (возможно, аккаунт угнан) |
| Администратор заблокировал пользователя (`blocked_at`) | завершить **все** |
| Пользователь удалён | сессии удалятся сами (`ON DELETE CASCADE`) |

---

## 7. Подтверждение email и сброс пароля

Обе операции работают через одноразовые токены из `email_action_tokens`. В письмо уходит ссылка с самим токеном, а в базу записывается только его хеш.

**Отправить письмо:**
```sql
-- погасить прежние неиспользованные токены того же типа
UPDATE email_action_tokens SET used_at = UTC_TIMESTAMP()
WHERE user_id = :user_id AND type = 'password_reset' AND used_at IS NULL;

INSERT INTO email_action_tokens (user_id, type, token_hash, expires_at)
VALUES (:user_id, 'password_reset', :hash, UTC_TIMESTAMP() + INTERVAL 1 HOUR);
-- ссылка: https://app/reset-password?token=<токен>
```

**Перейти по ссылке.** Пометить токен использованным атомарно, одним запросом, чтобы двойной клик не сработал дважды:
```sql
UPDATE email_action_tokens SET used_at = UTC_TIMESTAMP()
WHERE token_hash = :hash AND type = 'password_reset'
  AND used_at IS NULL AND expires_at > UTC_TIMESTAMP();
-- затронута 1 строка → токен валиден → можно менять пароль; 0 строк → «ссылка недействительна»
```
`user_id` для смены пароля берётся из этой же строки (`SELECT user_id … WHERE token_hash = :hash`).

После этого:
- `email_verify`: `UPDATE users SET email_verified_at = UTC_TIMESTAMP() WHERE id = :user_id`;
- `password_reset`: записать новый `password_hash` и завершить **все** сессии (см. п.6).

Рекомендуемые сроки: `email_verify` 24 часа, `password_reset` 1 час.

Если email не найден, «забыли пароль» всё равно отвечает «если такой email зарегистрирован, мы отправили письмо», чтобы по ответу нельзя было проверять, какие email есть в системе.

---

## 8. Обслуживание

Отозванные и истёкшие строки можно периодически удалять (например, раз в сутки), чтобы таблицы не росли:
```sql
DELETE FROM login_sessions
WHERE (revoked_at IS NOT NULL AND revoked_at < UTC_TIMESTAMP() - INTERVAL 30 DAY)
   OR expires_at < UTC_TIMESTAMP() - INTERVAL 30 DAY;

DELETE FROM email_action_tokens
WHERE expires_at < UTC_TIMESTAMP() - INTERVAL 7 DAY;
```
Запас в 30 дней нужен, чтобы по истории можно было разобраться, если пользователь сообщит о подозрительном входе.
