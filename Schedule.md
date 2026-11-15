# Расписание тренировок (дни недели проведения)

> План фичи: определение времени проведения тренировок — например,
> «понедельник, среда, пятница», «по субботам», «каждые N дней»
> или единичная тренировка на конкретную дату.

## Модель расписания

Каждый **день программы** (`program_days` — группа упражнений) может иметь
правило расписания. Режимы:

| Режим | Параметры | Пример |
|---|---|---|
| Еженедельно (`weekly`) | дни недели пн–вс + каждые N недель | пн, ср, пт |
| Ежедневно (`daily`) | каждые N дней (якорь — `start_date`) | через день |
| Однократно (`once`) | конкретная дата `once_date` | 20 ноября |
| Без расписания | правило отсутствует | свободный день |

Правило из `program_schedule_rules` при выборе программы пользователем
копируется в `user_schedule_rules` (дальше пользователь правит под себя).
Активных программ может быть несколько — календарь объединяет их планы.

## БД — миграция `00009_schedule_once.sql`

Таблицы `program_schedule_rules` / `user_schedule_rules` из `00004_programs.sql`
уже поддерживают daily/weekly. Добавляем единичный режим:

- `MODIFY frequency ENUM('daily','weekly','once') NOT NULL`
- `ADD COLUMN once_date DATE NULL` (после `interval_n`)
- Заменить `ck_*_sched_weekly` на общий CHECK:
  `(daily AND weekdays IS NULL) OR (weekly AND weekdays IS NOT NULL AND once_date IS NULL) OR (once AND weekdays IS NULL AND once_date IS NOT NULL)`
- Down: удалить `once`-правила, вернуть ENUM('daily','weekly') и старые CHECK

`remind_at` остаётся в схеме, в API/UI не выводится (на будущее).

## Backend

### 1. `internal/programs` — расписание в CRUD программ

**`detail.go`**
- Тип `Schedule {Frequency string, IntervalN uint8, Weekdays []string, OnceDate *string}`
- Поле `Schedule *Schedule` в `Day`
- Запрос к `program_schedule_rules` по `program_day_id IN (?)`;
  MySQL SET `weekdays` читается строкой `"mon,wed,fri"` → распарсить в массив

**`mutations.go`**
- `DayInput.Schedule *ScheduleInput`
- Валидация: `frequency ∈ {daily, weekly, once}`;
  weekly → weekdays 1..7 уникальных значений `mon..sun`, без `once_date`;
  daily → weekdays пуст, без `once_date`;
  once → `once_date` обязательна (валидная дата), weekdays пуст, interval = 1;
  `interval_n ≥ 1`
- Sentinel-ошибка `ErrInvalidSchedule` (+ маппинг 400 в `domainError` handler)
- `dayInsert` + `insertStructure`: INSERT правила после создания дня
- `Copy`: копировать правила (включая `once_date`) через `dayMap`
- Schedule опционален: `nil` = день без привязки к расписанию

### 2. Новый пакет `internal/schedule` (программы пользователя + план недели)

| Метод/путь | Назначение |
|---|---|
| `POST /api/v1/programs/:id/select` | тело `{start_date}` (default: сегодня): создаёт `user_programs` (is_active), копирует `program_schedule_rules` → `user_schedule_rules` |
| `GET /api/v1/schedule` | активные программы пользователя + правила (с названиями дней/программ) |
| `PUT /api/v1/schedule/rules/:id` | правка правила (frequency, interval_n, weekdays, once_date, is_enabled); владение через `user_programs.user_id` |
| `DELETE /api/v1/schedule/programs/:id` | деактивация программы пользователя |
| `GET /api/v1/schedule/week?date=YYYY-MM-DD` | план на неделю (пн–вс), содержащую `date` (default: текущая) |

**Расчёт плана** — чистая функция + unit-тесты:
- weekly: `weekday(d) ∈ weekdays` и `floor(days_since(start_date)/7) % interval_n == 0`
- daily: `days_since(start_date) % interval_n == 0`
- once: `d == once_date` (независимо от `start_date`)
- Ответ: `{date, user_program_id, program_id, program_day_id, day_name, program_name, position}`,
  сортировка по дате и позиции дня

**Вайринг:** `app.go` (сервис), `app.go:HTTPServer` + `server.go:NewRouter` (хендлер, роуты).

### 3. Тесты

- `programs_test.go`: расписание в create/update/copy/detail, ошибки валидации
- `schedule_test.go` (testutil): select → копирование правил; week-план для
  «пн/ср/пт», «только сб», «каждые N дней», «once»; деактивация; правка правила

## Frontend

- **`api/schedule.js`** (новый): get/updateRule/deactivate/week; `programsApi.select()` в `api/programs.js`
- **`stores/schedule.js`** (новый): активные программы, правила, план недели,
  навигация по неделям — по образцу `stores/programs.js`
- **`ProgramEditorView.vue`**: в карточке дня — режим расписания
  (еженедельно / каждые N дней / однократно / без расписания), чипсы дней недели
  (Пн–Вс), поле интервала, date-picker; поля в `form.days`, `buildBody()`, `loadForEdit()`
- **`ProgramDetailView.vue`**: бейджи дней недели у дней программы;
  кнопка «Начать программу» с датой старта (если не активна)
- **`CalendarView.vue`** (вместо заглушки): сетка недели пн–вс с навигацией
  ‹/›/«сегодня», карточки запланированных тренировок, подсветка «сегодня»;
  пустое состояние → CTA на «Тренировки»; блок активных программ с
  редактированием правил и деактивацией
- **`locales/ru.json`, `en.json`**: `weekdays.mon..sun`, `schedule.*`

## Проверка

- `make build && make vet && make test` (тест-БД: `make test-db-up`)
- `make front-lint && make front-build`
