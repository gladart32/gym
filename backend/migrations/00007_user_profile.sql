-- +goose Up
-- Профиль пользователя: статические данные (1:1 с users), история замеров
-- тела и выбранный инвентарь (значения из каталога equipment).

-- Статические поля профиля. Все опциональны: строка создаётся при первом
-- сохранении (upsert). fitness_level использует те же значения, что и
-- programs.level — основа для алгоритма подбора тренировок.
CREATE TABLE user_profiles (
    user_id          INT UNSIGNED NOT NULL,
    gender           ENUM('male','female') NULL,
    birth_date       DATE NULL,
    target_weight_kg DECIMAL(5,2) NULL,
    fitness_level    ENUM('beginner','intermediate','advanced') NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id),
    CONSTRAINT fk_user_profiles_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_user_profiles_birth_date CHECK (birth_date IS NULL OR birth_date >= '1900-01-01'),
    CONSTRAINT ck_user_profiles_target_weight CHECK (target_weight_kg IS NULL OR target_weight_kg BETWEEN 20 AND 500)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Профиль пользователя: пол, дата рождения, цели и уровень';

-- История замеров тела. Один замер на дату: повторный ввод перезаписывает
-- строку (upsert). «Текущий вес/рост» — последний замер, нигде не дублируется.
CREATE TABLE body_measurements (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id     INT UNSIGNED NOT NULL,
    measured_on DATE NOT NULL,
    weight_kg   DECIMAL(5,2) NULL,
    height_cm   DECIMAL(5,1) NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_body_measurements_user_date (user_id, measured_on),
    CONSTRAINT fk_body_measurements_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT ck_body_measurements_weight CHECK (weight_kg IS NULL OR weight_kg BETWEEN 20 AND 500),
    CONSTRAINT ck_body_measurements_height CHECK (height_cm IS NULL OR height_cm BETWEEN 50 AND 300),
    CONSTRAINT ck_body_measurements_data CHECK (weight_kg IS NOT NULL OR height_cm IS NOT NULL)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='История замеров веса и роста';

-- Инвентарь пользователя: ссылки на каталог equipment (совпадает с
-- exercises.equipment_id — база для фильтрации упражнений и программ).
CREATE TABLE user_equipment (
    user_id      INT UNSIGNED      NOT NULL,
    equipment_id SMALLINT UNSIGNED NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, equipment_id),
    CONSTRAINT fk_user_equipment_user      FOREIGN KEY (user_id)      REFERENCES users (id)     ON DELETE CASCADE,
    CONSTRAINT fk_user_equipment_equipment FOREIGN KEY (equipment_id) REFERENCES equipment (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Инвентарь пользователя';

-- +goose Down
DROP TABLE IF EXISTS user_equipment;
DROP TABLE IF EXISTS body_measurements;
DROP TABLE IF EXISTS user_profiles;
