-- +goose Up
-- Избранные упражнения пользователя (этап 3 раздела «Упражнения»).

CREATE TABLE exercise_favorites (
    user_id     INT UNSIGNED NOT NULL,
    exercise_id INT UNSIGNED NOT NULL,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, exercise_id),
    KEY ix_exercise_favorites_user_created (user_id, created_at),
    CONSTRAINT fk_ex_fav_user     FOREIGN KEY (user_id)     REFERENCES users (id)      ON DELETE CASCADE,
    CONSTRAINT fk_ex_fav_exercise FOREIGN KEY (exercise_id) REFERENCES exercises (id)  ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Избранные упражнения';

-- +goose Down

DROP TABLE IF EXISTS exercise_favorites;
