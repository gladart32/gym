-- +goose Up
-- Проведение тренировок.
-- При старте из программы план копируется в workout_* (planned_*),
-- пользователь отмечает факт; факт можно править после завершения.

CREATE TABLE workouts (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id         INT UNSIGNED    NOT NULL,
    user_program_id INT UNSIGNED    NULL,                     -- NULL = свободная тренировка
    program_day_id  INT UNSIGNED    NULL,
    name            VARCHAR(255)    NOT NULL,                 -- снимок названия дня / «Свободная тренировка»
    status          ENUM('in_progress','completed','cancelled') NOT NULL DEFAULT 'in_progress',
    started_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at     DATETIME        NULL,
    created_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    -- не больше одной незавершённой тренировки на пользователя
    active_user_id  INT UNSIGNED AS (IF(status = 'in_progress', user_id, NULL)) VIRTUAL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_workouts_one_active (active_user_id),
    KEY ix_workouts_user_started (user_id, started_at),
    KEY ix_workouts_user_program (user_program_id),
    KEY ix_workouts_program_day (program_day_id),
    CONSTRAINT fk_workouts_user         FOREIGN KEY (user_id)         REFERENCES users (id)         ON DELETE CASCADE,
    CONSTRAINT fk_workouts_user_program FOREIGN KEY (user_program_id) REFERENCES user_programs (id) ON DELETE SET NULL,
    CONSTRAINT fk_workouts_program_day  FOREIGN KEY (program_day_id)  REFERENCES program_days (id)  ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Проведённые / идущие тренировки';

-- Замена упражнения: exercise_id отличается от program_exercises.exercise_id.
-- Добавленное вне плана: program_exercise_id IS NULL.
CREATE TABLE workout_exercises (
    id                  BIGINT UNSIGNED   NOT NULL AUTO_INCREMENT,
    workout_id          BIGINT UNSIGNED   NOT NULL,
    exercise_id         INT UNSIGNED      NOT NULL,
    program_exercise_id INT UNSIGNED      NULL,
    position            TINYINT UNSIGNED  NOT NULL,
    rest_after_sec      SMALLINT UNSIGNED NULL,
    is_skipped          BOOLEAN           NOT NULL DEFAULT FALSE,
    PRIMARY KEY (id),
    KEY ix_workout_ex_workout (workout_id, position),
    KEY ix_workout_ex_exercise (exercise_id),
    KEY ix_workout_ex_program_ex (program_exercise_id),
    CONSTRAINT fk_workout_ex_workout    FOREIGN KEY (workout_id)          REFERENCES workouts (id)          ON DELETE CASCADE,
    CONSTRAINT fk_workout_ex_exercise   FOREIGN KEY (exercise_id)         REFERENCES exercises (id),
    CONSTRAINT fk_workout_ex_program_ex FOREIGN KEY (program_exercise_id) REFERENCES program_exercises (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Упражнения в тренировке';

CREATE TABLE workout_sets (
    id                   BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    workout_exercise_id  BIGINT UNSIGNED  NOT NULL,
    set_number           TINYINT UNSIGNED NOT NULL,
    -- план (снимок из program_sets на момент старта; NULL у добавленных подходов)
    planned_reps         SMALLINT UNSIGNED NULL,
    planned_weight_kg    DECIMAL(6,2)      NULL,
    planned_duration_sec INT UNSIGNED      NULL,
    planned_distance_m   INT UNSIGNED      NULL,
    planned_rest_sec     SMALLINT UNSIGNED NULL,
    -- факт
    reps                 SMALLINT UNSIGNED NULL,
    weight_kg            DECIMAL(6,2)      NULL,
    duration_sec         INT UNSIGNED      NULL,
    distance_m           INT UNSIGNED      NULL,
    completed_at         DATETIME          NULL,              -- NULL = подход не выполнен
    updated_at           DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_workout_sets_num (workout_exercise_id, set_number),
    CONSTRAINT fk_workout_sets_ex FOREIGN KEY (workout_exercise_id) REFERENCES workout_exercises (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Подходы тренировки: план и факт';

-- +goose Down
DROP TABLE IF EXISTS workout_sets;
DROP TABLE IF EXISTS workout_exercises;
DROP TABLE IF EXISTS workouts;
