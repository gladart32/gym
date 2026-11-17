# Проведение тренировки (workout session)

> План фичи: полноэкранный раздел проведения тренировки в ручном и
> автоматическом режимах. Не отображается в меню — вход кнопкой
> «Начать тренировку» из карточки «Следующая тренировка» на главной
> и из карточки дня в календаре.
>
> Статус: реализовано (v1), проверено end-to-end.

## Общая концепция

- Полноэкранный layout без сайдбара (фокус + удобно на телефоне в зале).
- Два режима, переключаются по ходу тренировки:
  - **Ручной**: «Начать подход» → таймер/запись → «Закончить подход» →
    отдых по `rest_sec` (можно «Пропустить» / «+20 сек») → сам запускаешь следующий.
  - **Авто**: после подтверждения подхода отдых стартует сам; по окончании
    отдыха следующий подход запускается сам (звук WebAudio + vibration).
    Time-based подход завершается по концу обратного отсчёта.
- Факт (вес/повторы/время/дистанция) вводится по ходу: план показывается
  крупно как цель, факт подставляется из плана и правится в один клик;
  вес/повторы запоминаются для следующих подходов.
- Отклонения от плана: пропустить упражнение, заменить на другое,
  добавить упражнение/подход по ходу (схема БД поддерживает).
- Устойчивость: каждое событие сразу уходит в API; после F5 состояние
  восстанавливается из БД (`GET /workouts/active`), указатель — по
  `completed_at`, таймеры отдыха стартуют заново с полной длительностью.
- Time-based (`tracking_type` `time` / `distance_time`): обратный отсчёт
  подхода, по окончании подход автоматически отмечается выполненным.
- Экран не гаснет во время тренировки: VueUse `useWakeLock`.
- По завершении — экран итогов: длительность, выполнено подходов,
  тоннаж (Σ вес×повторы), «Сохранить» / «Отменить».

## БД

Миграция `00005_workouts.sql` уже полностью поддерживает фичу, изменений нет:

- `workouts` — `status ENUM('in_progress','completed','cancelled')`,
  `started_at`/`finished_at`; одна активная тренировка на пользователя —
  virtual-колонка `active_user_id` + `UNIQUE uq_workouts_one_active`.
- `workout_exercises` — `program_exercise_id NULL` = добавлено по ходу;
  замена = `exercise_id ≠ program_exercises.exercise_id`; `is_skipped`;
  `rest_after_sec` — отдых ПОСЛЕ упражнения.
- `workout_sets` — план (`planned_reps/planned_weight_kg/planned_duration_sec/
  planned_distance_m/planned_rest_sec` — снапшот при старте) + факт
  (`reps/weight_kg/duration_sec/distance_m`) + `completed_at` (NULL = не выполнен).

## Backend — новый пакет `internal/workouts` (по образцу `schedule`)

Файлы: `internal/workouts/service.go` (Start/Active/loadSessionExercise),
`internal/workouts/mutations.go` (факты, подходы, упражнения, complete/cancel),
`internal/workouts/errors.go`, `internal/workouts/helpers.go`;
`internal/transport/http/handler/workouts.go`; вайринг в `app/app.go`,
`server/router.go`, `server/server.go`.

### API (`/api/v1/workouts`)

| Метод / путь | Назначение |
|---|---|
| `POST /workouts/start` | тело `{user_program_id, program_day_id}`: снапшот плана дня в `workout_*` (planned_*), статус `in_progress`; 409 `ErrActiveExists` если уже есть активная (unique `uq_workouts_one_active`), 404 если день/программа не принадлежат пользователю |
| `GET /workouts/active` | полное состояние (или `null`): тренировка + упражнения (с `exercise{name, tracking_type, equipment, gif_url, thumbnail_url, attribution}`) + подходы (план + факт + `completed_at`); для восстановления после F5 |
| `PATCH /workouts/sets/:id` | полная замена факта: `reps/weight_kg/duration_sec/distance_m` + `completed` (`true` → `completed_at=now`, `false` → NULL, отсутствует → без изменений); разрешён и после завершения тренировки; 400 при значениях вне диапазонов |
| `POST /workouts/exercises/:id/sets` | добавить подход по ходу (`set_number` = max+1); 422 при >50 подходов; только для `in_progress` |
| `PATCH /workouts/exercises/:id` | `is_skipped` и/или замена `exercise_id` (каталог проверяется); возвращает обновлённое упражнение; только для `in_progress` |
| `POST /workouts/exercises` | `{exercise_id, rest_after_sec?}` — добавить упражнение по ходу в активную тренировку (упражнение + 1 пустой подход); 422 при >50 упражнениях |
| `POST /workouts/complete` | `status=completed`, `finished_at=now()`; 404 если нет активной |
| `POST /workouts/cancel` | `status=cancelled` (запись остаётся с собранными фактами) |

Владелец проверяется через `user_id` из JWT (как в schedule). Названия
упражнений — по `Accept-Language` (как в programs/exercises).

### Семантика отдыха

- Между подходами — `set.rest_sec`; после последнего подхода упражнения —
  `exercise.rest_after_sec`; `NULL` = фаза отдыха пропускается (сразу дальше).
- Расчёт отдыха — на фронтенде (бэкенд хранит только факты).

## Frontend

- **Роут** (`router/index.js`): `/workout-session` вне `DefaultLayout`,
  отдельный `layouts/SessionLayout.vue` (без сайдбара),
  `meta: { requiresAuth: true, title: 'session.title' }`; в `AppSidebar`
  не добавляется. Отдельного router-guard нет: view вызывает
  `store.resume()` и при отсутствии активной тренировки сам делает
  `router.replace('dashboard')`.
- **`api/workouts.js`** (новый): start/active/updateSet/addSet/
  updateExercise/addExercise/complete/cancel.
- **`stores/workoutSession.js`** (новый) — машина состояний:

  ```
  phase:   loading → workout ⇄ summary   ('none' — активной нет, view уводит на дашборд)
  activity (внутри workout): idle ⇄ set → rest → (idle — ручной | set — авто)
  ```

  - Указатель `(exerciseIdx, setIdx)`, пропускает `is_skipped` и упражнения
    без подходов; `locateCurrent()`/`advance()` идут по первому
    неотмеченному (`completed_at IS NULL`) подходу.
  - `mode: 'manual' | 'auto'` — переключается по ходу; переход в авто во
    время ожидания сразу запускает подход.
  - Таймеры: общий (считается от `started_at` — переживает F5), отдых
    `restLeft/restTotal`, отсчёт time-based подхода `setLeft` (при
    достижении нуля подход завершается сам).
  - Факт: `fact` подставляется из плана (или сохранённого факта), вес/
    повторы переносятся с предыдущего выполненного подхода.
  - Действия: `startFromEntry` (старт; при 409 — `resume()` активной),
    `resume`, `beginSet`, `finishSet` (PATCH факта + отметка), `skipRest`,
    `extendRest` (+20 сек), `skipExercise`/`skipExerciseById`,
    `replaceExercise`/`replaceExerciseById`, `addExercise`, `addSet`,
    `jumpTo` (переход к любому подходу), `uncheckSet` (снятие отметки —
    можно вернуться из summary), `complete`, `cancel`, `reset`.
  - Звук WebAudio (`playBeep`) + `navigator.vibrate`: конец подхода,
    последние 3 сек отдыха, конец отдыха, финиш тренировки.
- **`views/WorkoutSessionView.vue`** + компоненты (`components/session/`):
  - `SessionHeader` — название, общий таймер, режим (Badge-клик + Switch),
    выход → диалог «Назад / Отменить тренировку / Завершить»;
  - `CurrentExerciseCard` — имя, gif (`gif_url` → fallback `thumbnail_url`),
    короткая атрибуция медиа («© Gym visual», как в `ExerciseCard`),
    инвентарь, отдых после упражнения; действия: Пропустить / Заменить
    (`ExercisePickerDialog`) / Добавить подход / список упражнений;
  - `SetList` — активный подход: факт с полями по `tracking_type` (маппинг
    как `setFields()` в редакторе программ) и степперами ±, кнопки
    «Начать/Закончить подход»; ниже таблица всех подходов «план → факт»,
    клик по строке — переход, клик по галке — снятие отметки;
  - `RestCard` — большой обратный отсчёт, прогресс-бар, «Что дальше»,
    «Пропустить», «+20 сек»;
  - `TimeSetCountdown` — обратный отсчёт time-based подхода (прогресс-бар),
    в ручном режиме кнопка «Закончить подход» досрочно;
  - `ExerciseSheet` (bottom Sheet) — все упражнения тренировки: переход,
    +подход, пропуск, замена, «Добавить упражнение»;
  - `SummaryScreen` — итоги: длительность, выполнено подходов X/Y, тоннаж
    (Σ вес×повторы выполненных), «Сохранить и выйти» (`/complete`) /
    «Отменить тренировку» (с подтверждением, `/cancel`).
- **`utils/format.js`** (новый): `formatClock` (MM:SS / H:MM:SS).
- **Точки входа**:
  - `DashboardView.vue`: кнопка «Начать» в каждой карточке `entry` —
    `sessionStore.startFromEntry(user_program_id, program_day_id)` →
    переход на `/workout-session`; при активной тренировке карточка
    заменяется баннером «У вас идёт тренировка» с кнопкой «Продолжить»;
  - `CalendarView.vue`: та же кнопка в карточках entries (только у
    «сегодня»; текст «Продолжить», если сессия активна).
- **`locales/ru.json`, `en.json`**: `session.*` + `common.back/cancel/close`.

## Тесты

`backend/internal/workouts/workouts_test.go` (testutil, по образцу
`schedule_test.go`), 7 сценариев:

- пустой `GET /active` сразу после регистрации;
- полный путь: старт → снапшот плана (название, `rest_after_sec`,
  `planned_*`, media URL, локализованное имя) → восстановимость через
  `active` → PATCH факта (сохранение + `completed_at`, затем сброс) →
  `complete` (статус в БД, `active` → null);
- 409 при втором старте;
- структурные изменения: skip, замена (`replaced`, новый `tracking_type`),
  добавление подхода (`set_number` 3), добавление упражнения (пустой
  подход, `program_exercise_id` null);
- отмена;
- cross-user доступ → 404 (sets/exercises/start чужой программы);
- валидация факта (вес/повторы вне диапазонов → 400).

## Порядок реализации

1. Бэкенд: пакет `workouts` (service + handler + роуты) → lint/build.
2. `api/workouts.js` + store-машина состояний.
3. Роут + `SessionLayout` + каркас view с восстановлением состояния.
4. Основной экран: `CurrentExerciseCard` + `SetList` + ручной режим + таймеры.
5. Авто-режим (звук/вибрация), `RestCard`, time-based отсчёт.
6. Отклонения: skip/replace/add.
7. Summary + complete/cancel, точки входа Dashboard/Calendar, баннер возврата.
8. i18n (ru/en), wake-lock, атрибуция медиа, полировка.

## Вне скоупа v1

- Свободная тренировка без программы (`user_program_id NULL` — схема готова, позже).
- История тренировок и страница статистики по ним.
- Офлайн-режим.

## Проверка

- `go build ./... && go vet ./...`; go test — только `-p 1`
  (`go test -p 1 ./...`): тест-пакеты чистят общую тест-БД
  (`make test-db-up`) и при параллельном прогоне гоняют друг друга
  (предсуществующая особенность, не связана с этой фичей).
- `make front-lint && make front-build`.
- Ручной сценарий (выполнен, браузер): старт с главной и из календаря,
  ручной режим (подход → факт → отдых → «+20 сек» → «Пропустить»),
  F5 в середине (позиция и общий таймер восстанавливаются), добавление
  подхода, переключение в авто (подход запускается сам после отдыха),
  time-based отсчёт (завершается сам), summary (длительность, 6/6,
  тоннаж), «Сохранить и выйти» → `completed` в БД, «Отменить тренировку»
  → `cancelled` в БД, баннер возврата на главной.
