-- +goose Up
-- Сессия входа = одна пара access/refresh на устройство. Сами токены НЕ хранятся.
-- access JWT содержит sid (= id сессии) и client_type; на каждый запрос
-- сервер сверяет, что сессия не отозвана (см. AUTH.md в проекте db).
-- refresh-токен хранится только в виде SHA-256 хеша; при обновлении ротируется.
CREATE TABLE login_sessions (
    id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id            INT UNSIGNED    NOT NULL,
    client_type        ENUM('web','mobile') NOT NULL,         -- флоу авторизации, не тип устройства
    refresh_token_hash BINARY(32)      NOT NULL,              -- SHA-256(текущего refresh)
    -- SHA-256 предыдущего (уже заменённого) refresh. Если его предъявили снова —
    -- токен украден и использован дважды: сессия отзывается целиком.
    prev_refresh_token_hash BINARY(32) NULL,
    device_name        VARCHAR(100)    NULL,                  -- 'iPhone 15', 'Galaxy Tab S9', 'Chrome'
    os                 VARCHAR(50)     NULL,                  -- 'iOS 18.1', 'Android 15', 'Windows 11'
    app_version        VARCHAR(20)     NULL,                  -- версия мобильного приложения: '1.2.0'
    user_agent         VARCHAR(512)    NULL,
    ip                 VARBINARY(16)   NULL,                  -- INET6_ATON()
    created_at         DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at       DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at         DATETIME        NOT NULL,
    revoked_at         DATETIME        NULL,                  -- logout / logout everywhere / смена пароля
    PRIMARY KEY (id),
    UNIQUE KEY uq_login_sessions_refresh (refresh_token_hash),
    KEY ix_login_sessions_prev_refresh (prev_refresh_token_hash),
    KEY ix_login_sessions_user (user_id, revoked_at),
    CONSTRAINT fk_login_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Сессии входа: хеш refresh-токена, срок, отзыв';

-- Одноразовые токены из ссылок в письмах: подтверждение email, сброс пароля.
CREATE TABLE email_action_tokens (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id     INT UNSIGNED    NOT NULL,
    type        ENUM('email_verify','password_reset') NOT NULL,
    token_hash  BINARY(32)      NOT NULL,                     -- SHA-256(token из письма)
    expires_at  DATETIME        NOT NULL,
    used_at     DATETIME        NULL,
    created_at  DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_email_tokens_hash (token_hash),
    KEY ix_email_tokens_user_type (user_id, type),
    CONSTRAINT fk_email_tokens_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Одноразовые токены из писем: подтверждение email, сброс пароля';

-- +goose Down
DROP TABLE IF EXISTS email_action_tokens;
DROP TABLE IF EXISTS login_sessions;
