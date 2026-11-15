-- +goose Up
-- Единичные тренировки на конкретную дату: frequency='once' + once_date.
-- daily   : weekdays IS NULL, once_date IS NULL
-- weekly  : weekdays IS NOT NULL, once_date IS NULL
-- once    : weekdays IS NULL, once_date IS NOT NULL

ALTER TABLE program_schedule_rules
    MODIFY frequency ENUM('daily','weekly','once') NOT NULL,
    ADD COLUMN once_date DATE NULL AFTER interval_n,
    DROP CHECK ck_prog_sched_weekly,
    ADD CONSTRAINT ck_prog_sched_freq CHECK (
        (frequency = 'daily'  AND weekdays IS NULL     AND once_date IS NULL)
        OR (frequency = 'weekly' AND weekdays IS NOT NULL AND once_date IS NULL)
        OR (frequency = 'once'   AND weekdays IS NULL     AND once_date IS NOT NULL)
    );

ALTER TABLE user_schedule_rules
    MODIFY frequency ENUM('daily','weekly','once') NOT NULL,
    ADD COLUMN once_date DATE NULL AFTER interval_n,
    DROP CHECK ck_user_sched_weekly,
    ADD CONSTRAINT ck_user_sched_freq CHECK (
        (frequency = 'daily'  AND weekdays IS NULL     AND once_date IS NULL)
        OR (frequency = 'weekly' AND weekdays IS NOT NULL AND once_date IS NULL)
        OR (frequency = 'once'   AND weekdays IS NULL     AND once_date IS NOT NULL)
    );

-- +goose Down
DELETE FROM user_schedule_rules WHERE frequency = 'once';
DELETE FROM program_schedule_rules WHERE frequency = 'once';

ALTER TABLE program_schedule_rules
    DROP CHECK ck_prog_sched_freq,
    MODIFY frequency ENUM('daily','weekly') NOT NULL,
    DROP COLUMN once_date,
    ADD CONSTRAINT ck_prog_sched_weekly CHECK (frequency = 'daily' OR weekdays IS NOT NULL);

ALTER TABLE user_schedule_rules
    DROP CHECK ck_user_sched_freq,
    MODIFY frequency ENUM('daily','weekly') NOT NULL,
    DROP COLUMN once_date,
    ADD CONSTRAINT ck_user_sched_weekly CHECK (frequency = 'daily' OR weekdays IS NOT NULL);
