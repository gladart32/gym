-- +goose Up
-- Снапшот дня в личном правиле: расписание начатой программы живёт
-- независимо от правок программы (правка Update пересоздаёт program_days,
-- а каскад fk_user_sched_day стирал правила пользователей).

ALTER TABLE user_schedule_rules
    ADD COLUMN day_name VARCHAR(255) NOT NULL AFTER program_day_id,
    ADD COLUMN day_position TINYINT UNSIGNED NOT NULL AFTER day_name;

UPDATE user_schedule_rules r
JOIN program_days d ON d.id = r.program_day_id
SET r.day_name = d.name,
    r.day_position = d.position;

ALTER TABLE user_schedule_rules
    DROP FOREIGN KEY fk_user_sched_day;

-- +goose Down
ALTER TABLE user_schedule_rules
    ADD CONSTRAINT fk_user_sched_day FOREIGN KEY (program_day_id)
    REFERENCES program_days (id) ON DELETE CASCADE;

ALTER TABLE user_schedule_rules
    DROP COLUMN day_position,
    DROP COLUMN day_name;
