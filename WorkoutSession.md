# Проведение тренировки (workout session)

> План фичи: полноэкранный раздел проведения тренировки в ручном и
> автоматическом режимах. Не отображается в меню — вход кнопкой
> «Начать тренировку» из карточки «Следующая тренировка» на главной
> и из карточки дня в календаре.

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

Файлы: `internal/workouts/service.go`,
`internal/transport/http/handler/workouts.go`; вайринг в `app/app.go`,
`server/router.go`.

### API (`/api/v1/workouts`)

| Метод / путь | Назначение |
|---|---|
| `POST /workouts/start` | тело `{user_program_id, program_day_id}`: снапшот плана дня в `workout_*` (planned_*), статус `in_progress`; 409 если уже есть активная |
| `GET /workouts/active` | полное состояние: тренировка + упражнения (с `exercise{name, tracking_type, media}`) + подходы (план + факт + `completed_at`); для восстановления после F5 |
| `PATCH /workouts/sets/:id` | факт подхода: `reps/weight_kg/duration_sec/distance_m` + отметка/сброс `completed_at` |
| `POST /workouts/exercises/:id/sets` | добавить подход по ходу (`set_number` = max+1) |
| `PATCH /workouts/exercises/:id` | `is_skipped` или замена `exercise_id` |
| `POST /workouts/exercises` | добавить упражнение по ходу (упражнение + 1 пустой подход) |
| `POST /workouts/complete` | `status=completed`, `finished_at=now()` |
| `POST /workouts/cancel` | `status=cancelled` |

Владелец проверяется через `user_id` из JWT (как в schedule).

### Семантика отдыха

- Между подходами — `set.rest_sec`; после последнего подхода упражнения —
  `exercise.rest_after_sec`; `NULL` = фаза отдыха пропускается (сразу дальше).
- Расчёт отдыха — на фронтенде (бэкенд хранит только факты).

## Frontend

- **Роут** (`router/index.js`): `/workout-session`, name `workout-session`,
  отдельный `layouts/SessionLayout.vue` (без сайдбара),
  `meta: { requiresAuth: true, title: 'session.title' }`; в `AppSidebar`
  не добавлять. Guard: нет активной тренировки → редирект на `/`.
- **`api/workouts.js`** (новый): start/active/patchSet/addSet/patchExercise/
  addExercise/complete/cancel.
- **`stores/workoutSession.js`** (новый) — машина состояний:

  ```
  idle → overview → set_ready ⇄ set_running → rest → (next set / next exercise) → summary
  ```

  - Указатель `(exerciseIndex, setIndex)`, пропускает `is_skipped` и упражнения без подходов.
  - `mode: 'manual' | 'auto'` — переключается по ходу.
  - Таймеры: общий таймер тренировки (от `started_at`); отдых `rest_sec`;
    обратный отсчёт подхода `duration_sec` для time-based.
  - Персистентность: все события сразу в API; восстановление из `active`.
- **`views/WorkoutSessionView.vue`** + компоненты (`components/session/`):
  - `SessionHeader` — название, общий таймер, Switch «Авто/Ручной», выход
    (меню: завершить/отменить);
  - `CurrentExerciseCard` — имя, gif (`media.gif_url` → fallback
    `thumbnail_url`, attribution как в `ExerciseCard.vue`), описание техники;
  - `SetList` — подходы текущего упражнения: план крупно, факт в
    редактируемых полях по `tracking_type` (маппинг как `setFields()` в
    `ProgramEditorView.vue`), галка выполнения;
  - `RestCard` — большой обратный отсчёт, «Что дальше», «Пропустить», «+20 сек»;
  - `TimeSetCountdown` — круговой отсчёт для time-based подходов
    (+ дистанция для `distance_time`);
  - `ExerciseSheet` — список упражнений тренировки: пропустить / заменить
    (переиспользовать `ExercisePickerDialog.vue`) / добавить;
  - `SummaryScreen` — итоги: длительность, выполнено подходов, тоннаж,
    «Сохранить» (`/complete`) / «Отменить» (`/cancel`).
- **Точки входа**:
  - `DashboardView.vue`: кнопка «Начать» в каждой карточке `entry`
    (query `user_program_id` + `program_day_id`); при активной тренировке —
    баннер «Вернуться к тренировке»;
  - `CalendarView.vue`: та же кнопка в карточках entries (только у «сегодня»);
    оба вызывают `POST /workouts/start` → переход на `/workout-session`.
- **`locales/ru.json`, `en.json`**: `session.*`.

## Порядок реализации

1. Бэкенд: пакет `workouts` (service + handler + роуты) → lint/build, curl-проверка.
2. `api/workouts.js` + store-машина состояний.
3. Роут + `SessionLayout` + каркас view с восстановлением состояния.
4. Основной экран: `CurrentExerciseCard` + `SetList` + ручной режим + таймеры.
5. Авто-режим (звук/вибрация), `RestCard`, time-based отсчёт.
6. Отклонения: skip/replace/add.
7. Summary + complete/cancel, точки входа Dashboard/Calendar, баннер возврата.
8. i18n (ru/en), wake-lock, полировка.

## Вне скоупа v1

- Свободная тренировка без программы (`user_program_id NULL` — схема готова, позже).
- История тренировок и страница статистики по ним.
- Офлайн-режим.

## Проверка

- `make build && make vet && make test` (тест-БД: `make test-db-up`)
- `make front-lint && make front-build`
- Ручной сценарий: старт с главной и из календаря, полный проход по
  тренировке в обоих режимах, F5 в середине, skip/replace/add, complete/cancel.
