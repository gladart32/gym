-- +goose Up
-- Программы тренировок и расписание.
--   owner_id IS NULL  -> готовая (каталожная) программа, управляет admin
--   owner_id NOT NULL -> личная программа пользователя
--   Структура: программа -> дни -> упражнения -> подходы (каждый подход отдельно).

CREATE TABLE programs (
    id               INT UNSIGNED NOT NULL AUTO_INCREMENT,
    owner_id         INT UNSIGNED NULL,
    copied_from_id   INT UNSIGNED NULL,                       -- если скопирована из готовой
    name             VARCHAR(255) NOT NULL,
    description      TEXT         NULL,
    author_name      VARCHAR(255) NULL,                       -- необязательное авторство
    level            ENUM('beginner','intermediate','advanced') NULL,
    is_published     BOOLEAN      NOT NULL DEFAULT FALSE,     -- для готовых: видна ли в каталоге
    created_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY ix_programs_owner (owner_id),
    KEY ix_programs_catalog (is_published, level),
    CONSTRAINT fk_programs_owner  FOREIGN KEY (owner_id)       REFERENCES users (id)    ON DELETE CASCADE,
    CONSTRAINT fk_programs_copied FOREIGN KEY (copied_from_id) REFERENCES programs (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Программы тренировок (готовые и личные)';

-- День программы = группа упражнений («День A — грудь/трицепс»).
CREATE TABLE program_days (
    id          INT UNSIGNED NOT NULL AUTO_INCREMENT,
    program_id  INT UNSIGNED NOT NULL,
    position    TINYINT UNSIGNED NOT NULL,
    name        VARCHAR(255) NOT NULL,
    PRIMARY KEY (id),
    KEY ix_program_days_program (program_id, position),
    CONSTRAINT fk_program_days_program FOREIGN KEY (program_id) REFERENCES programs (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Дни (группы упражнений) программы';

CREATE TABLE program_exercises (
    id              INT UNSIGNED      NOT NULL AUTO_INCREMENT,
    program_day_id  INT UNSIGNED      NOT NULL,
    exercise_id     INT UNSIGNED      NOT NULL,
    position        TINYINT UNSIGNED  NOT NULL,
    rest_after_sec  SMALLINT UNSIGNED NULL,                   -- отдых после упражнения (перед следующим)
    PRIMARY KEY (id),
    KEY ix_program_ex_day (program_day_id, position),
    KEY ix_program_ex_exercise (exercise_id),
    CONSTRAINT fk_program_ex_day      FOREIGN KEY (program_day_id) REFERENCES program_days (id) ON DELETE CASCADE,
    CONSTRAINT fk_program_ex_exercise FOREIGN KEY (exercise_id)    REFERENCES exercises (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Упражнения в дне программы';

CREATE TABLE program_sets (
    id                  INT UNSIGNED      NOT NULL AUTO_INCREMENT,
    program_exercise_id INT UNSIGNED      NOT NULL,
    set_number          TINYINT UNSIGNED  NOT NULL,
    reps                SMALLINT UNSIGNED NULL,
    weight_kg           DECIMAL(6,2)      NULL,               -- < 0 = помощь (гравитрон)
    duration_sec        INT UNSIGNED      NULL,
    distance_m          INT UNSIGNED      NULL,
    rest_sec            SMALLINT UNSIGNED NULL,               -- отдых после подхода
    PRIMARY KEY (id),
    UNIQUE KEY uq_program_sets_num (program_exercise_id, set_number),
    CONSTRAINT fk_program_sets_ex FOREIGN KEY (program_exercise_id) REFERENCES program_exercises (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Плановые подходы (каждый подход отдельно)';

-- Расписание по умолчанию для программы (в основном для готовых).
-- frequency=daily  : каждые interval_n дней
-- frequency=weekly : каждые interval_n недель по дням из weekdays
CREATE TABLE program_schedule_rules (
    id              INT UNSIGNED NOT NULL AUTO_INCREMENT,
    program_day_id  INT UNSIGNED NOT NULL,
    frequency       ENUM('daily','weekly') NOT NULL,
    interval_n      TINYINT UNSIGNED NOT NULL DEFAULT 1,
    weekdays        SET('mon','tue','wed','thu','fri','sat','sun') NULL,
    remind_at       TIME             NULL,                    -- локальное время пользователя
    PRIMARY KEY (id),
    KEY ix_prog_sched_day (program_day_id),
    CONSTRAINT fk_prog_sched_day FOREIGN KEY (program_day_id) REFERENCES program_days (id) ON DELETE CASCADE,
    CONSTRAINT ck_prog_sched_weekly CHECK (frequency = 'daily' OR weekdays IS NOT NULL),
    CONSTRAINT ck_prog_sched_interval CHECK (interval_n >= 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Расписание по умолчанию для дней программы';

-- Программа, выбранная пользователем (готовая или своя).
CREATE TABLE user_programs (
    id          INT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id     INT UNSIGNED NOT NULL,
    program_id  INT UNSIGNED NOT NULL,
    start_date  DATE         NOT NULL,                        -- якорь для interval_n > 1
    end_date    DATE         NULL,
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY ix_user_programs_user (user_id, is_active),
    KEY ix_user_programs_program (program_id),
    CONSTRAINT fk_user_programs_user    FOREIGN KEY (user_id)    REFERENCES users (id)    ON DELETE CASCADE,
    CONSTRAINT fk_user_programs_program FOREIGN KEY (program_id) REFERENCES programs (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Выбранные пользователем программы';

-- Расписание пользователя: копируется из program_schedule_rules при выборе программы,
-- дальше пользователь правит под себя. Используется для напоминаний.
CREATE TABLE user_schedule_rules (
    id               INT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_program_id  INT UNSIGNED NOT NULL,
    program_day_id   INT UNSIGNED NOT NULL,
    frequency        ENUM('daily','weekly') NOT NULL,
    interval_n       TINYINT UNSIGNED NOT NULL DEFAULT 1,
    weekdays         SET('mon','tue','wed','thu','fri','sat','sun') NULL,
    remind_at        TIME         NULL,
    is_enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    PRIMARY KEY (id),
    KEY ix_user_sched_up (user_program_id),
    KEY ix_user_sched_day (program_day_id),
    CONSTRAINT fk_user_sched_up  FOREIGN KEY (user_program_id) REFERENCES user_programs (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_sched_day FOREIGN KEY (program_day_id)  REFERENCES program_days (id)  ON DELETE CASCADE,
    CONSTRAINT ck_user_sched_weekly CHECK (frequency = 'daily' OR weekdays IS NOT NULL),
    CONSTRAINT ck_user_sched_interval CHECK (interval_n >= 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Расписание пользователя (напоминания)';

-- +goose Down
DROP TABLE IF EXISTS user_schedule_rules;
DROP TABLE IF EXISTS user_programs;
DROP TABLE IF EXISTS program_schedule_rules;
DROP TABLE IF EXISTS program_sets;
DROP TABLE IF EXISTS program_exercises;
DROP TABLE IF EXISTS program_days;
DROP TABLE IF EXISTS programs;
