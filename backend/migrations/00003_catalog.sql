-- +goose Up
-- Каталог упражнений (наполняется seed-импортом, правки вносит только admin).

CREATE TABLE body_parts (
    id       SMALLINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code     VARCHAR(50)       NOT NULL,                      -- ключ из датасета: 'upper legs'
    name_en  VARCHAR(100)      NOT NULL,
    name_ru  VARCHAR(100)      NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_body_parts_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Части тела';

CREATE TABLE equipment (
    id       SMALLINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code     VARCHAR(50)       NOT NULL,
    name_en  VARCHAR(100)      NOT NULL,
    name_ru  VARCHAR(100)      NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_equipment_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Оборудование';

-- Синонимы (abs/abdominals, lats/latissimus dorsi…) схлопываются при импорте
-- в одну запись по полю canonical из translations/ru/muscles.json.
CREATE TABLE muscles (
    id       SMALLINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code     VARCHAR(50)       NOT NULL,
    name_en  VARCHAR(100)      NOT NULL,
    name_ru  VARCHAR(100)      NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_muscles_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Мышцы (нормализованные)';

CREATE TABLE exercises (
    id               INT UNSIGNED      NOT NULL AUTO_INCREMENT,
    source           VARCHAR(32)       NOT NULL DEFAULT 'manual', -- 'exercises-dataset' | 'manual'
    external_id      VARCHAR(32)       NULL,                      -- id в источнике ('0001')
    body_part_id     SMALLINT UNSIGNED NOT NULL,
    equipment_id     SMALLINT UNSIGNED NOT NULL,
    target_muscle_id SMALLINT UNSIGNED NOT NULL,
    -- какие поля подхода обязательны:
    --   weight_reps   : вес + повторы
    --   reps          : повторы (+ опц. вес — доп. отягощение / помощь со знаком минус)
    --   time          : время (+ опц. вес)
    --   distance_time : время (+ опц. дистанция, вес)
    tracking_type    ENUM('weight_reps','reps','time','distance_time') NOT NULL DEFAULT 'weight_reps',
    is_active        BOOLEAN           NOT NULL DEFAULT TRUE,         -- скрыть из каталога без удаления
    created_at       DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_exercises_source (source, external_id),
    KEY ix_exercises_body_part (body_part_id),
    KEY ix_exercises_equipment (equipment_id),
    KEY ix_exercises_target (target_muscle_id),
    CONSTRAINT fk_exercises_body_part FOREIGN KEY (body_part_id)     REFERENCES body_parts (id),
    CONSTRAINT fk_exercises_equipment FOREIGN KEY (equipment_id)     REFERENCES equipment (id),
    CONSTRAINT fk_exercises_target    FOREIGN KEY (target_muscle_id) REFERENCES muscles (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Упражнения';

CREATE TABLE exercise_translations (
    exercise_id  INT UNSIGNED NOT NULL,
    lang         CHAR(2)      NOT NULL,                       -- 'ru', 'en'
    name         VARCHAR(255) NOT NULL,
    instructions TEXT         NULL,
    steps        JSON         NULL,                           -- ["шаг 1", "шаг 2", …]
    PRIMARY KEY (exercise_id, lang),
    KEY ix_ex_tr_lang_name (lang, name),
    FULLTEXT KEY ft_ex_tr_name (name) WITH PARSER ngram,      -- поиск по подстроке, в т.ч. кириллица
    CONSTRAINT fk_ex_tr_exercise FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Названия и инструкции на разных языках';

CREATE TABLE exercise_secondary_muscles (
    exercise_id INT UNSIGNED      NOT NULL,
    muscle_id   SMALLINT UNSIGNED NOT NULL,
    PRIMARY KEY (exercise_id, muscle_id),
    KEY ix_ex_sec_muscle (muscle_id),
    CONSTRAINT fk_ex_sec_exercise FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE,
    CONSTRAINT fk_ex_sec_muscle   FOREIGN KEY (muscle_id)   REFERENCES muscles (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Вспомогательные мышцы упражнения';

-- Медиа отдельно, чтобы источник можно было заменить (лицензия Gym visual:
-- рядом с медиа показывать attribution, разрешение только 180×180).
CREATE TABLE exercise_media (
    id          INT UNSIGNED      NOT NULL AUTO_INCREMENT,
    exercise_id INT UNSIGNED      NOT NULL,
    type        ENUM('gif','thumbnail','image','video') NOT NULL,
    path        VARCHAR(500)      NOT NULL,                   -- относительный путь / URL
    width       SMALLINT UNSIGNED NULL,
    height      SMALLINT UNSIGNED NULL,
    source      VARCHAR(64)       NULL,                       -- 'gymvisual'
    attribution VARCHAR(255)      NULL,                       -- '© Gym visual — https://gymvisual.com/'
    sort_order  TINYINT UNSIGNED  NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY ix_media_exercise (exercise_id, type, sort_order),
    CONSTRAINT fk_media_exercise FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='GIF, миниатюры и прочие медиа упражнений';

-- +goose Down
DROP TABLE IF EXISTS exercise_media;
DROP TABLE IF EXISTS exercise_secondary_muscles;
DROP TABLE IF EXISTS exercise_translations;
DROP TABLE IF EXISTS exercises;
DROP TABLE IF EXISTS muscles;
DROP TABLE IF EXISTS equipment;
DROP TABLE IF EXISTS body_parts;
