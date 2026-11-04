-- +goose Up
-- Статистика (VIEW поверх фактических подходов).
-- Учитываются только выполненные подходы (completed_at IS NOT NULL)
-- в идущих или завершённых тренировках (не отменённых).

CREATE VIEW v_done_sets AS
SELECT
    w.user_id,
    w.id                    AS workout_id,
    w.started_at,
    DATE(w.started_at)      AS workout_date,
    we.exercise_id,
    e.tracking_type,
    e.target_muscle_id,
    ws.id                   AS set_id,
    ws.set_number,
    ws.reps,
    ws.weight_kg,
    ws.duration_sec,
    ws.distance_m,
    CASE WHEN ws.weight_kg > 0 AND ws.reps > 0 THEN ws.weight_kg * ws.reps END AS volume_kg,
    -- расчётный 1ПМ по Эпли; только для рабочего диапазона 1..12 повторов
    CASE WHEN e.tracking_type = 'weight_reps' AND ws.weight_kg > 0 AND ws.reps BETWEEN 1 AND 12
         THEN ROUND(ws.weight_kg * (1 + ws.reps / 30), 2) END                   AS e1rm_kg
FROM workout_sets ws
JOIN workout_exercises we ON we.id = ws.workout_exercise_id
JOIN workouts w           ON w.id  = we.workout_id
JOIN exercises e          ON e.id  = we.exercise_id
WHERE ws.completed_at IS NOT NULL
  AND w.status <> 'cancelled';

-- Сводка по каждой тренировке
CREATE VIEW v_workout_summary AS
SELECT
    w.id AS workout_id,
    w.user_id,
    w.name,
    w.status,
    w.started_at,
    w.finished_at,
    TIMESTAMPDIFF(SECOND, w.started_at, w.finished_at) AS duration_sec,
    COUNT(DISTINCT d.exercise_id)  AS exercises_done,
    COUNT(d.set_id)                AS sets_done,
    COALESCE(SUM(d.reps), 0)       AS total_reps,
    COALESCE(SUM(d.volume_kg), 0)  AS total_volume_kg
FROM workouts w
LEFT JOIN v_done_sets d ON d.workout_id = w.id
GROUP BY w.id;

-- Личные рекорды по упражнению
CREATE VIEW v_personal_records AS
SELECT
    user_id,
    exercise_id,
    MAX(weight_kg)    AS max_weight_kg,
    MAX(reps)         AS max_reps,
    MAX(volume_kg)    AS max_set_volume_kg,
    MAX(e1rm_kg)      AS best_e1rm_kg,
    MAX(duration_sec) AS max_duration_sec,
    MAX(distance_m)   AS max_distance_m,
    COUNT(*)          AS sets_total,
    MAX(started_at)   AS last_done_at
FROM v_done_sets
GROUP BY user_id, exercise_id;

-- Прогресс по упражнению по дням (для графиков)
CREATE VIEW v_exercise_progress AS
SELECT
    user_id,
    exercise_id,
    workout_date,
    MAX(weight_kg)    AS max_weight_kg,
    MAX(reps)         AS max_reps,
    MAX(e1rm_kg)      AS best_e1rm_kg,
    SUM(volume_kg)    AS volume_kg,
    MAX(duration_sec) AS max_duration_sec,
    SUM(distance_m)   AS distance_m,
    COUNT(*)          AS sets_done
FROM v_done_sets
GROUP BY user_id, exercise_id, workout_date;

-- Недельная активность пользователя (неделя ISO, YEARWEEK mode 3)
CREATE VIEW v_weekly_stats AS
SELECT
    user_id,
    YEARWEEK(started_at, 3)     AS iso_week,
    MIN(workout_date)           AS week_first_workout,
    COUNT(DISTINCT workout_id)  AS workouts,
    COUNT(*)                    AS sets_done,
    COALESCE(SUM(reps), 0)      AS total_reps,
    COALESCE(SUM(volume_kg), 0) AS total_volume_kg
FROM v_done_sets
GROUP BY user_id, YEARWEEK(started_at, 3);

-- Нагрузка по целевым мышцам по неделям
CREATE VIEW v_muscle_weekly_volume AS
SELECT
    user_id,
    target_muscle_id            AS muscle_id,
    YEARWEEK(started_at, 3)     AS iso_week,
    COUNT(*)                    AS sets_done,
    COALESCE(SUM(volume_kg), 0) AS volume_kg
FROM v_done_sets
GROUP BY user_id, target_muscle_id, YEARWEEK(started_at, 3);

-- +goose Down
DROP VIEW IF EXISTS v_muscle_weekly_volume;
DROP VIEW IF EXISTS v_weekly_stats;
DROP VIEW IF EXISTS v_exercise_progress;
DROP VIEW IF EXISTS v_personal_records;
DROP VIEW IF EXISTS v_workout_summary;
DROP VIEW IF EXISTS v_done_sets;
