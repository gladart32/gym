-- +goose Up
-- Пользователи приложения. Каталог управляется администратором (role = 'admin').
CREATE TABLE users (
    id                INT UNSIGNED     NOT NULL AUTO_INCREMENT,
    email             VARCHAR(255)     NOT NULL,              -- collation _ci => уникальность без учёта регистра
    password_hash     VARCHAR(255)     NOT NULL,              -- bcrypt / argon2id
    name              VARCHAR(100)     NULL,
    role              ENUM('user','admin') NOT NULL DEFAULT 'user',
    timezone          VARCHAR(64)      NOT NULL DEFAULT 'Europe/Moscow', -- для напоминаний по расписанию
    email_verified_at DATETIME         NULL,
    blocked_at        DATETIME         NULL,
    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Пользователи приложения';

-- +goose Down
DROP TABLE IF EXISTS users;
