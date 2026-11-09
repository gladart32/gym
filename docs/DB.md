# База данных Gym

MySQL 8.4 (InnoDB, `utf8mb4`, сортировка `utf8mb4_0900_ai_ci`). Схема — goose-миграции [`backend/migrations/`](../backend/migrations/) (применяются сервисом `migrate` в compose или `make migrate-up`).

Общие правила:
- Все `DATETIME` хранятся в **UTC** (сервер запущен с `default-time-zone = '+00:00'`). Перевод в часовой пояс пользователя делает приложение по `users.timezone`.
- Сортировка `_ai_ci` не различает регистр, а также «е» и «ё»: `Test@Mail.ru` = `test@mail.ru`, «лёжа» = «лежа».
- Вес хранится в килограммах (`DECIMAL(6,2)`), время в секундах, дистанция в метрах.

## Содержание

1. [Схема связей](#схема-связей)
2. [Пользователи и аутентификация](#1-пользователи-и-аутентификация)
3. [Каталог упражнений](#2-каталог-упражнений)
4. [Программы тренировок и расписание](#3-программы-тренировок-и-расписание)
5. [Проведение тренировок](#4-проведение-тренировок)
6. [Статистика (VIEW)](#5-статистика-view)
7. [Картинки и GIF](#картинки-и-gif)
8. [Перенос и резервное копирование базы](#перенос-и-резервное-копирование-базы)
9. [Локальный запуск](#локальный-запуск)

---

## Схема связей

```
users ─┬─< login_sessions        (сессии входа)
       ├─< email_action_tokens   (токены из писем)
       ├─< programs (owner_id, личные программы)
       ├─< user_programs ─< user_schedule_rules >─ program_days
       ├─| user_profiles                     (профиль, 1:1)
       ├─< body_measurements               (история веса/роста)
       └─< user_equipment >─ equipment     (инвентарь пользователя)

body_parts ─┐
equipment  ─┼─< exercises ─┬─< exercise_translations
muscles    ─┘              ├─< exercise_secondary_muscles >─ muscles
                           └─< exercise_media

programs ─< program_days ─┬─< program_exercises ─< program_sets
                          └─< program_schedule_rules
program_exercises >─ exercises
workout_exercises >─ exercises, program_exercises
```

`A ─< B` означает «одна запись A, много записей B».

---

## 1. Пользователи и аутентификация

### `users`: пользователи приложения

Все, кто зарегистрирован в приложении. Роль `admin` даёт право управлять каталогом упражнений и готовыми программами.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `email` | VARCHAR(255) UNIQUE | Логин. Уникален без учёта регистра |
| `password_hash` | VARCHAR(255) | Хеш пароля (bcrypt / argon2id), сам пароль не хранится |
| `name` | VARCHAR(100) NULL | Отображаемое имя |
| `role` | ENUM(`user`,`admin`) | Роль, по умолчанию `user` |
| `timezone` | VARCHAR(64) | Часовой пояс IANA (`Europe/Moscow`), нужен для напоминаний по расписанию |
| `email_verified_at` | DATETIME NULL | Когда подтверждён email; NULL, если ещё не подтверждён |
| `blocked_at` | DATETIME NULL | Когда заблокирован; NULL, если активен |
| `created_at`, `updated_at` | DATETIME | Служебные метки |

### `login_sessions`: сессии входа (refresh-токены)

Подробно, с примерами запросов (вход, проверка, обновление, список сессий в ЛК, завершение сессий): [AUTH.md](AUTH.md).

Одна запись соответствует одному входу с одного устройства (паре access/refresh JWT). Через неё можно отозвать токены: выход с устройства, выход со всех устройств, сброс при смене пароля.

Как это работает:
- В payload access-JWT кладутся `sid` (= `login_sessions.id`) и `client_type`. Ключи payload одинаковы для web и mobile.
- Refresh-токен хранится только в виде SHA-256-хеша. При обновлении он ротируется: в той же строке записывается новый хеш, а старый переносится в `prev_refresh_token_hash`.
- Если кто-то предъявил **предыдущий**, уже заменённый refresh, значит, токен украден и им воспользовались дважды. Сессия отзывается целиком ([AUTH.md, п.3](AUTH.md#зачем-prev_refresh_token_hash-обнаружение-кражи-refresh-токена)).
- Сессия недействительна, если `revoked_at IS NOT NULL` или `expires_at < NOW()`.
- Access-токен в базе не хранится: он проверяется по подписи, а срок жизни записан в нём самом (`exp`). Отзыв работает через `sid`: если сессия отозвана, её access-токен тоже отклоняется.

> **Почему `BINARY(32)`.** Это не длина токена, а размер SHA-256-хеша. Хеш всегда занимает 32 байта, какой бы длины ни был JWT. Сервер считает `SHA-256(присланный refresh)` и ищет сессию по `refresh_token_hash`. Сами токены не хранятся: если утечёт дамп базы, по хешам войти нельзя (принцип тот же, что с паролями).

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | Идентификатор сессии (`sid` в JWT) |
| `user_id` | FK → `users` | Владелец. При удалении пользователя сессии удаляются |
| `client_type` | ENUM(`web`,`mobile`) | Флоу авторизации, а не тип устройства: планшет с приложением будет `mobile`, планшет с браузером `web` |
| `refresh_token_hash` | BINARY(32) UNIQUE | SHA-256 текущего refresh-токена. По нему сервер находит сессию при обновлении |
| `prev_refresh_token_hash` | BINARY(32) NULL, индекс | SHA-256 предыдущего (уже заменённого) refresh-токена. Нужен **для обнаружения кражи**: если его предъявили снова, сессия отзывается. NULL до первой ротации |
| `device_name` | VARCHAR(100) NULL | Устройство для списка в ЛК: `iPhone 15`, `Galaxy Tab S9`, `Chrome`. Мобильное приложение передаёт при входе, для web определяется из `user_agent` |
| `os` | VARCHAR(50) NULL | ОС: `iOS 18.1`, `Android 15`, `Windows 11` |
| `app_version` | VARCHAR(20) NULL | Версия мобильного приложения (`1.2.0`); у web NULL |
| `user_agent` | VARCHAR(512) NULL | Исходная строка User-Agent |
| `ip` | VARBINARY(16) NULL | IP-адрес (`INET6_ATON()` / `INET6_NTOA()`) |
| `created_at` | DATETIME | Время входа |
| `last_used_at` | DATETIME | Последнее обновление токена («был в сети» в списке ЛК) |
| `expires_at` | DATETIME | Срок жизни refresh-токена (сессии) |
| `revoked_at` | DATETIME NULL | Когда отозвана: выход, завершение из ЛК, смена пароля, обнаруженная кража |

### `email_action_tokens`: одноразовые токены из писем

Ссылки подтверждения email и сброса пароля. Хранится только хеш токена. После использования заполняется `used_at`, и повторно токен не принимается.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `user_id` | FK → `users` | |
| `type` | ENUM(`email_verify`,`password_reset`) | Назначение токена |
| `token_hash` | BINARY(32) UNIQUE | SHA-256 токена из письма |
| `expires_at` | DATETIME | До какого момента действует |
| `used_at` | DATETIME NULL | Когда использован |
| `created_at` | DATETIME | |

### `user_profiles`: профиль пользователя (1:1 с `users`)

Статические данные профиля. Всё необязательно: строка создаётся при первом сохранении (upsert), пустые поля — «не указано». Отдельная таблица вместо колонок в `users`, чтобы не смешивать auth и профиль. `fitness_level` использует те же значения, что и `programs.level`, — основа для алгоритма подбора тренировок.

| Колонка | Тип | Описание |
|---|---|---|
| `user_id` | INT UNSIGNED PK, FK → `users` | 1:1 с пользователем |
| `gender` | ENUM(`male`,`female`) NULL | Пол; нужен для расчёта калорий и норм |
| `birth_date` | DATE NULL | Дата рождения; CHECK — не раньше `1900-01-01`, «не в будущем» проверяет приложение |
| `target_weight_kg` | DECIMAL(5,2) NULL | Целевой вес, 20–500 кг |
| `fitness_level` | ENUM(`beginner`,`intermediate`,`advanced`) NULL | Уровень физической подготовки |
| `created_at`, `updated_at` | DATETIME | Служебные метки |

Опыт тренировок отдельным полем не хранится — его отражает `fitness_level`.

### `body_measurements`: история замеров веса и роста

Вес и рост — не поля профиля, а история: один замер на дату, повторный ввод за тот же день перезаписывает строку (upsert по `uq_body_measurements_user_date`). «Текущий вес/рост» — последний замер по дате, нигде не дублируется. Пример: 01.11 — 85 кг / 180 см; 08.11 — 84.5 кг / 180 см; 15.11 — 84 кг / 180 см.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `user_id` | FK → `users` | Владелец |
| `measured_on` | DATE | Дата замера; UNIQUE вместе с `user_id` — один замер в день |
| `weight_kg` | DECIMAL(5,2) NULL | Вес, 20–500 кг; CHECK — хотя бы один из веса/роста задан |
| `height_cm` | DECIMAL(5,1) NULL | Рост, 50–300 см |
| `created_at`, `updated_at` | DATETIME | Служебные метки |

### `user_equipment`: инвентарь пользователя

Какое оборудование есть у пользователя. Ссылки на каталог `equipment` (те же `id`, что в `exercises.equipment_id`) — в будущем фильтр упражнений и программ по доступному инвентарю.

| Колонка | Тип | Описание |
|---|---|---|
| `user_id` | INT UNSIGNED, PK-часть, FK → `users` | |
| `equipment_id` | SMALLINT UNSIGNED, PK-часть, FK → `equipment` | Позиция каталога |
| `created_at` | DATETIME | Когда добавлено |

---

## 2. Каталог упражнений

Каталог наполняется импортом из [hasaneyldrm/exercises-dataset](https://github.com/hasaneyldrm/exercises-dataset) и русских переводов (`data/translations/ru/`) командой `make seed-catalog` из корня репо (импортёр — `backend/cmd/importer`). Пользователи каталог не меняют, правки вносит только администратор.

### `body_parts`, `equipment`, `muscles`: справочники

Части тела (10), оборудование (28), мышцы (40). Структура у всех трёх одинаковая.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | SMALLINT UNSIGNED PK | |
| `code` | VARCHAR(50) UNIQUE | Ключ из датасета (`upper legs`, `barbell`, `quads`), по нему работает импорт |
| `name_en` | VARCHAR(100) | Название на английском |
| `name_ru` | VARCHAR(100) | Название на русском |

В датасете 50 названий мышц, но часть из них синонимы (`abs`/`abdominals`, `lats`/`latissimus dorsi`, `quads`/`quadriceps`…). При импорте синонимы склеиваются в одну запись по полю `canonical` из `data/translations/ru/muscles.json`.

### `exercises`: упражнения

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | Внутренний id, используется во всех ссылках |
| `source` | VARCHAR(32) | Откуда упражнение: `exercises-dataset` или `manual` (добавлено админом) |
| `external_id` | VARCHAR(32) NULL | id в источнике (`0001`). Пара (`source`, `external_id`) уникальна, по ней импорт находит уже загруженные упражнения |
| `body_part_id` | FK → `body_parts` | Часть тела |
| `equipment_id` | FK → `equipment` | Оборудование |
| `target_muscle_id` | FK → `muscles` | Основная (целевая) мышца |
| `tracking_type` | ENUM | Что записывается в подходе, см. ниже |
| `is_active` | BOOLEAN | `FALSE`: упражнение скрыто из каталога, но не удалено (на него могут ссылаться программы и история) |
| `created_at`, `updated_at` | DATETIME | |

**`tracking_type`** определяет, какие поля подхода приложение показывает и какие из них обязательны:

| Значение | Обязательно | Необязательно | Примеры | Кол-во |
|---|---|---|---|---|
| `weight_reps` | вес, повторы | — | жим, присед, тяга блока | 858 |
| `reps` | повторы | вес (доп. отягощение; отрицательный вес означает помощь, гравитрон) | отжимания, подтягивания, пресс | 371 |
| `time` | время | вес | планка, растяжка, вис | 82 |
| `distance_time` | время | дистанция, вес | бег, велотренажёр, прогулка фермера | 13 |

Тип определяется при импорте автоматически (`backend/cmd/importer/tracking.go`), поэтому в разметке возможны ошибки. Их правит администратор.

### `exercise_translations`: названия и инструкции

На каждое упражнение приходится одна строка на язык. Сейчас загружены `ru` и `en`.

| Колонка | Тип | Описание |
|---|---|---|
| `exercise_id` | FK → `exercises`, PK | |
| `lang` | CHAR(2), PK | Язык: `ru`, `en` |
| `name` | VARCHAR(255) | Название |
| `instructions` | TEXT NULL | Инструкция одним текстом |
| `steps` | JSON NULL | Та же инструкция по шагам: `["шаг 1", "шаг 2", …]` |

Индексы: `(lang, name)` для сортировки и FULLTEXT `ngram` по `name` для поиска по части слова, в том числе кириллицей:
```sql
SELECT exercise_id, name FROM exercise_translations
WHERE lang = 'ru' AND MATCH(name) AGAINST('штанг' IN BOOLEAN MODE);
```
FULLTEXT-индекс InnoDB обновляется при `COMMIT`, поэтому строки из незакоммиченной транзакции в поиске не видны.

### `exercise_secondary_muscles`: вспомогательные мышцы

Связь «многие ко многим» между упражнениями и мышцами. Целевая мышца хранится в `exercises.target_muscle_id` и сюда не дублируется.

| Колонка | Тип | Описание |
|---|---|---|
| `exercise_id` | FK → `exercises`, PK | |
| `muscle_id` | FK → `muscles`, PK | |

### `exercise_media`: GIF, миниатюры и другие медиа

Медиа вынесены в отдельную таблицу, чтобы источник можно было заменить, не трогая упражнения (у GIF своя лицензия, см. [Картинки и GIF](#картинки-и-gif)).

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `exercise_id` | FK → `exercises` | |
| `type` | ENUM(`gif`,`thumbnail`,`image`,`video`) | Вид файла |
| `path` | VARCHAR(500) | Относительный путь (`videos/0001-2gPfomN.gif`) или URL |
| `width`, `height` | SMALLINT NULL | Размер в пикселях (у датасета 180×180) |
| `source` | VARCHAR(64) NULL | Источник: `gymvisual` |
| `attribution` | VARCHAR(255) NULL | Подпись, которую нужно показывать рядом с медиа: `© Gym visual — https://gymvisual.com/` |
| `sort_order` | TINYINT | Порядок, если файлов одного типа несколько |

При повторном импорте с `-update` пересоздаются только записи с `source = 'gymvisual'`. Медиа, загруженные администратором, не затрагиваются.

---

## 3. Программы тренировок и расписание

Программа состоит из **дней** (групп упражнений), дни состоят из **упражнений**, упражнения из **подходов**. Каждый подход задаётся отдельно, поэтому можно описать, например, пирамиду 12×40 → 10×45 → 8×50.

### `programs`: программы

Здесь хранятся и готовые программы из каталога, и личные программы пользователей:
- `owner_id IS NULL`: **готовая** программа, её создаёт и публикует администратор;
- `owner_id IS NOT NULL`: **личная** программа пользователя. Её можно составить с нуля или скопировать из готовой.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `owner_id` | FK → `users` NULL | Владелец личной программы; NULL у готовой |
| `copied_from_id` | FK → `programs` NULL | Из какой программы сделана копия |
| `name` | VARCHAR(255) | Название |
| `description` | TEXT NULL | Описание |
| `author_name` | VARCHAR(255) NULL | Автор программы, необязательно |
| `level` | ENUM(`beginner`,`intermediate`,`advanced`) NULL | На какой уровень подготовки рассчитана |
| `is_published` | BOOLEAN | Видна ли готовая программа в каталоге (`FALSE` = черновик) |
| `created_at`, `updated_at` | DATETIME | |

### `program_days`: дни программы

Группа упражнений на одно занятие («День A: грудь и трицепс»).

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `program_id` | FK → `programs` | При удалении программы дни удаляются |
| `position` | TINYINT | Порядок дня в программе |
| `name` | VARCHAR(255) | Название дня |

### `program_exercises`: упражнения дня

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `program_day_id` | FK → `program_days` | |
| `exercise_id` | FK → `exercises` | Упражнение из каталога. Удалить упражнение, которое используется в программах, нельзя |
| `position` | TINYINT | Порядок в дне |
| `rest_after_sec` | SMALLINT NULL | Отдых **после упражнения**, перед следующим |

### `program_sets`: плановые подходы

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `program_exercise_id` | FK → `program_exercises` | |
| `set_number` | TINYINT | Номер подхода, уникален в пределах упражнения |
| `reps` | SMALLINT NULL | Повторы (точное число) |
| `weight_kg` | DECIMAL(6,2) NULL | Вес; отрицательный означает помощь |
| `duration_sec` | INT NULL | Время (для `time` / `distance_time`) |
| `distance_m` | INT NULL | Дистанция (для `distance_time`) |
| `rest_sec` | SMALLINT NULL | Отдых **после подхода** |

Какие из полей `reps` / `weight_kg` / `duration_sec` / `distance_m` заполнять, определяет `exercises.tracking_type`.

### `program_schedule_rules`: расписание программы по умолчанию

Рекомендуемое расписание готовой программы. Когда пользователь выбирает программу, расписание копируется в `user_schedule_rules`.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `program_day_id` | FK → `program_days` | Какой день программы назначается |
| `frequency` | ENUM(`daily`,`weekly`) | Каждый день или каждую неделю |
| `interval_n` | TINYINT ≥ 1 | Повторять каждые N дней или недель (по умолчанию 1) |
| `weekdays` | SET(`mon`…`sun`) NULL | Дни недели; обязательны при `weekly` (проверяется CHECK) |
| `remind_at` | TIME NULL | Время напоминания в часовом поясе пользователя |

Пример: «День A по понедельникам, B по средам, C по пятницам» записывается тремя правилами `weekly` с `weekdays` = `mon`, `wed`, `fri`.

### `user_programs`: выбранные пользователем программы

Связывает пользователя с программой (готовой или своей), по которой он занимается.

| Колонка | Тип | Описание |
|---|---|---|
| `id` | INT UNSIGNED PK | |
| `user_id` | FK → `users` | |
| `program_id` | FK → `programs` | |
| `start_date` | DATE | Дата начала, точка отсчёта для `interval_n > 1` |
| `end_date` | DATE NULL | Дата окончания |
| `is_active` | BOOLEAN | Занимается ли сейчас по этой программе |
| `created_at` | DATETIME | |

### `user_schedule_rules`: расписание пользователя

Личная копия расписания, которую пользователь правит под себя. Используется **для напоминаний**: тренировку пользователь запускает сам.

Колонки те же, что в `program_schedule_rules`, плюс:

| Колонка | Тип | Описание |
|---|---|---|
| `user_program_id` | FK → `user_programs` | Вместо привязки к программе |
| `is_enabled` | BOOLEAN | Включено ли напоминание |

---

## 4. Проведение тренировок

Когда тренировка начинается по дню программы, приложение копирует план в `workout_exercises` и `workout_sets` (поля `planned_*`). Пользователь отмечает фактические вес и повторы. Факт можно изменить и после завершения. Пропустить упражнение, заменить его или добавить новое тоже можно.

### `workouts`: тренировки

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `user_id` | FK → `users` | |
| `user_program_id` | FK → `user_programs` NULL | По какой программе; NULL у свободной тренировки |
| `program_day_id` | FK → `program_days` NULL | По какому дню |
| `name` | VARCHAR(255) | Название на момент старта (снимок): если программу потом изменят, история не изменится |
| `status` | ENUM(`in_progress`,`completed`,`cancelled`) | Идёт / завершена / отменена |
| `started_at` | DATETIME | Начало |
| `finished_at` | DATETIME NULL | Окончание |
| `created_at`, `updated_at` | DATETIME | |
| `active_user_id` | виртуальная колонка | `user_id`, если тренировка идёт, иначе NULL. На ней уникальный индекс: **у пользователя может быть только одна незавершённая тренировка** |

Если программу или день удалят, ссылки в тренировке обнулятся (`SET NULL`), а сама тренировка и её подходы останутся в истории.

### `workout_exercises`: упражнения тренировки

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `workout_id` | FK → `workouts` | |
| `exercise_id` | FK → `exercises` | Какое упражнение выполнялось фактически |
| `program_exercise_id` | FK → `program_exercises` NULL | Из какого пункта плана взято; NULL, если добавлено по ходу |
| `position` | TINYINT | Порядок |
| `rest_after_sec` | SMALLINT NULL | Отдых после упражнения |
| `is_skipped` | BOOLEAN | Упражнение пропущено |

**Замена упражнения** определяется так: `exercise_id` ≠ `program_exercises.exercise_id`.

### `workout_sets`: подходы (план и факт)

| Колонка | Тип | Описание |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `workout_exercise_id` | FK → `workout_exercises` | |
| `set_number` | TINYINT | Номер подхода, уникален в пределах упражнения |
| `planned_reps`, `planned_weight_kg`, `planned_duration_sec`, `planned_distance_m`, `planned_rest_sec` | NULL | **План**: снимок из `program_sets` на момент старта. NULL у подходов, добавленных по ходу |
| `reps`, `weight_kg`, `duration_sec`, `distance_m` | NULL | **Факт**: что пользователь выполнил |
| `completed_at` | DATETIME NULL | Когда подход отмечен выполненным; NULL, если не выполнен. Из разницы между отметками можно получить фактический отдых |
| `updated_at` | DATETIME | |

---

## 5. Статистика (VIEW)

Статистика считается только по **фактически выполненным** подходам: `completed_at IS NOT NULL` в неотменённых тренировках. Если пользователь не отметил вес и повторы, по этому подходу ничего не выводится.

| VIEW | Что показывает | Ключевые колонки |
|---|---|---|
| `v_done_sets` | Все выполненные подходы с пользователем, датой и типом упражнения. На ней построены остальные VIEW | `user_id`, `workout_date`, `exercise_id`, `reps`, `weight_kg`, `volume_kg` (вес × повторы), `e1rm_kg` (расчётный 1ПМ по Эпли, для 1–12 повторов) |
| `v_workout_summary` | Итоги каждой тренировки | `duration_sec`, `exercises_done`, `sets_done`, `total_reps`, `total_volume_kg` |
| `v_personal_records` | Личные рекорды по каждому упражнению | `max_weight_kg`, `max_reps`, `max_set_volume_kg`, `best_e1rm_kg`, `max_duration_sec`, `max_distance_m`, `last_done_at` |
| `v_exercise_progress` | Прогресс по упражнению по дням (для графиков) | `workout_date`, `max_weight_kg`, `best_e1rm_kg`, `volume_kg` |
| `v_weekly_stats` | Активность по неделям (неделя ISO, `YEARWEEK(…, 3)`) | `iso_week`, `workouts`, `sets_done`, `total_volume_kg` |
| `v_muscle_weekly_volume` | Нагрузка на целевые мышцы по неделям | `muscle_id`, `iso_week`, `sets_done`, `volume_kg` |

Пример: рекорды пользователя с русскими названиями упражнений:
```sql
SELECT t.name, r.max_weight_kg, r.best_e1rm_kg, r.max_reps
FROM v_personal_records r
JOIN exercise_translations t ON t.exercise_id = r.exercise_id AND t.lang = 'ru'
WHERE r.user_id = ?
ORDER BY r.last_done_at DESC;
```

---

## Картинки и GIF

### Где лежат

Файлы находятся в клоне датасета, в базе хранятся только относительные пути к ним:

```
data/exercises-dataset/          ← git clone https://github.com/hasaneyldrm/exercises-dataset
├── videos/   1324 × GIF-анимация, 180×180, ~126 МБ   (exercise_media.type = 'gif')
└── images/   1324 × JPG-миниатюра, 180×180, ~12 МБ   (exercise_media.type = 'thumbnail')
```

Значения `exercise_media.path` выглядят как `videos/0001-2gPfomN.gif` и `images/0001-2gPfomN.jpg`. Пути считаются **от корня датасета**.

Папка `data/exercises-dataset/` добавлена в `.gitignore` и в репозиторий не попадает. На новой машине её нужно склонировать заново:
```bash
make dataset-fetch   # из корня репо (git clone --depth 1 …/exercises-dataset data/exercises-dataset)
```

### Как разместить в другом проекте

1. Скопируйте обе папки в каталог статики проекта, сохранив их имена:
   ```bash
   mkdir -p /path/to/app/public/media
   cp -r data/exercises-dataset/videos data/exercises-dataset/images /path/to/app/public/media/
   ```
2. Раздавайте каталог по HTTP. Полный URL строится как `<база> + path`:
   - **через приложение** (Go):
     ```go
     http.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir("public/media"))))
     // URL = "/media/" + path  →  /media/videos/0001-2gPfomN.gif
     ```
   - **через nginx**:
     ```nginx
     location /media/ {
         alias /path/to/app/public/media/;
         expires 30d;
         add_header Cache-Control "public, immutable";
     }
     ```
   - **через CDN или S3**: загрузите папки в бакет, а базовый URL задайте в конфиге приложения (`MEDIA_BASE_URL=https://cdn.example.com/media/`).
3. Базовый URL лучше держать в **конфиге приложения**, а не в базе: тогда при переезде на другой хостинг или CDN в БД ничего менять не придётся. Если всё-таки нужен полный путь в БД, используйте флаг импорта `-media-prefix`:
    ```bash
    cd backend && go run ./cmd/importer -update -media-prefix "/media/"   # path = /media/videos/0001-….gif
    ```

### Лицензия медиа: обязательно

GIF и миниатюры принадлежат **Gym visual** и распространяются в датасете с разрешения правообладателя ([NOTICE.md](data/exercises-dataset/NOTICE.md)). Условия:
- рядом с медиа всегда показывать подпись **© Gym visual — https://gymvisual.com/** (она хранится в `exercise_media.attribution`);
- использовать только в разрешении **180×180**: апскейл и перерисовка запрещены. Уменьшать отображение или слегка увеличивать через CSS можно, сами файлы менять нельзя;
- для публичного проекта нужно изучить [условия Gym visual](https://gymvisual.com/content/3-terms-and-conditions-of-use) и при необходимости получить лицензию.

Тексты, названия и структура данных распространяются под MIT, ограничений на них нет.

---

## Перенос и резервное копирование базы

Есть два способа. Выбор зависит от того, нужны ли пользовательские данные.

### Способ 1. Чистая база: миграции + импорт каталога

Подходит для нового окружения, где нужен только каталог упражнений без пользователей и истории.

1. Разверните стек Gym (миграции применятся сами сервисом `migrate`):
   ```bash
   make dataset-fetch   # клон датасета (GIF/миниатюры)
   make media-sync      # медиа в backend/media
   make up              # mysql, redis, migrate, app, worker, nginx
   ```
2. Загрузите каталог:
   ```bash
   make seed-catalog    # нужен запущенный mysql; переводы уже в data/translations/ru (в git)
   ```
   Для другого сервера MySQL импортёр запускается напрямую с флагом `-dsn`
   (`cd backend && go run ./cmd/importer -dsn '…'`, полный список флагов — `go run ./cmd/importer -h`).
3. Медиа: `make media-sync` уже скопировал их в `backend/media` — их раздаёт nginx (см. [выше](#как-разместить-в-другом-проекте)).

### Способ 2. Полная копия: дамп

Переносит всё как есть: каталог с правками администратора, пользователей, программы и историю тренировок.

1. Снимите дамп:
   ```bash
   docker exec gym-mysql sh -c \
     'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --routines --triggers \
      --set-gtid-purged=OFF --default-character-set=utf8mb4 gym' \
     | sed -E 's/DEFINER=`[^`]+`@`[^`]+`//g' > gym_dump.sql
   ```
   `sed` убирает `DEFINER=…` из определений VIEW. Без этого восстановление на сервере, где нет пользователя `root@%`, упадёт с ошибкой прав.
2. Восстановите на целевом сервере (база `gym` создана, миграции применены как в способе 1):
   ```bash
   mysql -h HOST -P PORT -u gym -p gym < gym_dump.sql
   ```
3. Только каталог, без пользователей и истории:
   ```bash
   docker exec gym-mysql sh -c \
     'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --no-create-info \
      --set-gtid-purged=OFF --default-character-set=utf8mb4 gym \
      body_parts equipment muscles exercises exercise_translations exercise_secondary_muscles exercise_media' \
     > catalog_data.sql
   ```
   Это только данные. На целевой стороне сначала примените миграции, затем загрузите `catalog_data.sql`.
4. Разместите медиа: дамп содержит только пути, сами файлы нужно скопировать отдельно.

Требования к целевому серверу: MySQL **8.0.16+** (CHECK-ограничения, FULLTEXT `ngram`, `utf8mb4_0900_ai_ci`); рекомендуется 8.4. Время в базе хранится в UTC. Убедитесь, что приложение подключается с `loc=UTC` (Go) или эквивалентной настройкой.

---

## Локальный запуск

| Параметр | Значение |
|---|---|
| Хост / порт | `127.0.0.1:3306` (публикуется контейнером `gym-mysql`) |
| База | `gym` |
| Пользователь | из `.env` в корне (`MYSQL_USER` / `MYSQL_PASSWORD`, дефолт `gym`/`gym`) |
| Volume | `gym_mysql_data` |
| Тест-база | `gym-mysql-test` :3307, база `gym_test` (`make test-db-up`, compose-профиль `test`) |

```bash
make up             # поднять стек (миграции — сервисом migrate)
make seed-catalog   # загрузить каталог упражнений
make stop           # остановить все контейнеры (включая тест-БД и asynqmon)
make down           # убрать контейнеры и сети (данные в volume сохранятся)
docker exec -it gym-mysql mysql -ugym -pgym gym --default-character-set=utf8mb4
```

Схема применяется goose-миграциями (`backend/migrations/`) при каждом старте стека: сервис `migrate` доводит базу до актуальной версии на существующем volume. Изменения схемы — только новыми миграциями (`make migrate-create name=имя`).
