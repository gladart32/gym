# Exercises

План реализации раздела «Упражнения» (каталог с фильтрами, поиском, пагинацией и визуализацией). Составлен по итогам исследования бэкенда/фронтенда и схемы БД (`docs/DB.md`).

## Принятые решения

- Список — **нумерованная пагинация** (кнопки страниц, shadcn Pagination)
- Детали упражнения — **диалог** (Dialog), без отдельной страницы
- В карточках — **GIF всегда** (180×180, `loading="lazy"`, чтобы грузились только видимые)
- Фильтры часть тела / оборудование / мышца — **мультивыбор** (чекбоксы)
- Вся группа `/api/v1/exercises` — за `middleware.Auth` (приложение полностью авторизованное)
- Избранное (лайки) — отдельным этапом 3

## Доступные фильтры (по данным БД)

| Фильтр | Значений | Источник |
|---|---|---|
| Поиск по названию | — | FULLTEXT ngram `ft_ex_tr_name` по `exercise_translations.name` (готовый индекс, поддерживает кириллицу по подстроке) |
| Часть тела | 10 | `body_parts` → `exercises.body_part_id` |
| Оборудование | 28 | `equipment` → `exercises.equipment_id` |
| Целевая мышца | 40 | `muscles` → `exercises.target_muscle_id` |
| Тип записи результата | 4 | `exercises.tracking_type`: `weight_reps` / `reps` / `time` / `distance_time` |
| Мой инвентарь | toggle | `user_equipment` (уже редактируется в профиле) + всегда включается `equipment.code = 'body weight'` («Собственный вес») |
| Избранное | toggle | новая таблица `exercise_favorites` (этап 3) |

Уровня сложности у упражнений **нет** — отсутствует и в датасете, и в схеме (уровни есть только в `user_profiles.fitness_level` и `programs.level`).

Сортировка: по имени (на выбранном языке, индекс `ix_ex_tr_lang_name`) / по новизне.

---

## Этап 1 — Бэкенд каталога

### Домен `backend/internal/exercises/`

Паттерн — как `internal/profile`: `Service{db, log}` сам является репозиторием, сырой SQL в const-строках, sqlx (`SelectContext` / `GetContext` / `ExecContext`), теги `json` + `db`.

- `service.go` — `Service`, `NewService(db, log)`
- `list.go` — список с динамическим WHERE:
  - база: `FROM exercises e JOIN exercise_translations t ON t.exercise_id = e.id AND t.lang = ?` (язык из `Accept-Language`) `WHERE e.is_active = 1`
  - `q` — `MATCH(t.name) AGAINST(? IN BOOLEAN MODE)` с экранированием булевых операторов (`+ - * " …`); фолбэк на `LIKE` для 1-символьных запросов (ngram-токен = 2 символа)
  - `body_part_ids`, `equipment_ids`, `muscle_ids` — `IN (…)` из распарсенных uint (инъекции нет: значения типизированы)
  - `tracking_types` — `IN (…)` по whitelist enum
  - `my_equipment=1` — `e.equipment_id IN (SELECT equipment_id FROM user_equipment WHERE user_id = ?) OR e.equipment_id = (SELECT id FROM equipment WHERE code = 'body weight')`
  - сортировка `sort=name|newest` (+ направление), пагинация `LIMIT/OFFSET` + отдельный `COUNT(*)` с теми же условиями
  - медиа: миниатюра и GIF одной строкой (подзапрос/JOIN по `exercise_media`, `sort_order`), нормализация пути — если `path` уже начинается с `/media/`, использовать как есть, иначе `/media/` + path
- `detail.go` — детальная карточка: перевод (name, instructions, steps JSON), справочники, вторичные мышцы (`exercise_secondary_muscles`), все медиа
- `filters.go` — локализованные справочники для панели фильтров: body_parts, equipment, muscles, tracking_types (имя выбирается по `Accept-Language` из `name_ru`/`name_en`)

### Эндпоинты (все за Auth)

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/v1/exercises` | Список с фильтрами |
| GET | `/api/v1/exercises/:id` | Детальная карточка |
| GET | `/api/v1/exercises/filters` | Справочники для панели фильтров |

Query-параметры списка: `q`, `body_part_ids`, `equipment_ids`, `muscle_ids` (comma-separated id), `tracking_types` (comma-separated enum), `my_equipment=1`, `sort=name|newest`, `order=asc|desc`, `page` (≥1), `per_page` (дефолт 20, макс 50).

Контракт ответа списка:

```json
{
  "items": [
    {
      "id": 1,
      "name": "Жим штанги лежа",
      "body_part":       { "id": 3, "name": "Грудь" },
      "equipment":       { "id": 5, "name": "Штанга" },
      "target_muscle":   { "id": 7, "name": "Грудь" },
      "tracking_type": "weight_reps",
      "media": { "gif_url": "/media/videos/0001-….gif", "thumbnail_url": "/media/images/0001-….jpg", "attribution": "© Gym visual — https://gymvisual.com/" }
    }
  ],
  "total": 1324, "page": 1, "per_page": 20, "pages": 67
}
```

Названия сущностей (части тела, оборудование, мышцы, упражнения) локализует бэкенд по `Accept-Language` — на фронте не дублируются.

### Подключение

- Хендлер `internal/transport/http/handler/exercises.go` (`NewExercises(log, svc)`), парсинг query по образцу параметров (`strconv.ParseUint`), ответы только через `response.Success/Error`
- Роуты в `internal/server/router.go` (группа `/api/v1/exercises` + `middleware.Auth`)
- DI: поле `Exercises` в `internal/app/app.go` (`New()` — создание, `HTTPServer()` — хендлер в `NewRouter`)

### Тесты

`internal/exercises/exercises_test.go` (внешний пакет `package exercises_test`, по образцу `auth_test.go`): `testutil.Start(t)` + фикстуры напрямую в БД (упражнение, переводы, медиа, справочники). Проверить:

- список без фильтров + пагинация (total/page/pages, границы per_page)
- каждый фильтр отдельно и в комбинации
- поиск по подстроке кириллицей
- `my_equipment`: юзер с выбранным инвентарём / без него; «Собственный вес» попадает всегда
- деталь по id, 404 на несуществующее / `is_active=0`
- 401 без токена

---

## Этап 2 — Фронтенд

- Установить компоненты: `npx shadcn-vue add checkbox badge pagination dialog select scroll-area switch`
- `src/api/exercises.js` — по образцу `profile.js`: `{ __requiresAuth: true }`, unwrap `resp.data.data`; методы `list(params)`, `get(id)`, `filters()`
- **`views/ExercisesView.vue`** (переписать заглушку):
  - тулбар: поиск (debounce через VueUse `refDebounced`/`useDebounceFn`), select сортировки
  - панель фильтров: десктоп — колонка слева, мобилка — Sheet; секции: часть тела, оборудование (28, ScrollArea), мышцы (40, ScrollArea), тип записи, переключатель «Мой инвентарь» (показывается, только если в профиле выбран инвентарь — из `profileApi.equipment()`), «Избранное» (этап 3)
  - счётчик активных фильтров (Badge) + кнопка «Сбросить»
  - фильтры / страница / поиск синхронизируются с URL query (`router.replace`) — шаримые ссылки
- **`components/exercises/ExerciseCard.vue`**: GIF (`loading="lazy"`, `decoding="async"`, 180×180), название, бейджи части тела и оборудования, компактная подпись © Gym visual (лицензия требует attribution рядом с медиа); клик — открытие деталей; сердце (этап 3)
- **`components/exercises/ExerciseDetailDialog.vue`**: GIF + полная attribution, инструкция по шагам (`steps`), целевая и вторичные мышцы, оборудование, тип записи
- Пагинация: shadcn Pagination внизу сетки
- Состояния: Skeleton при загрузке, empty state («ничего не найдено» + сброс), ошибки — `toast.error` (vue-sonner)
- i18n: расширить неймспейс `exercises` в `src/locales/ru.json` и `en.json` (оба файла обязательно)

---

## Этап 3 — Избранное (отдельным заходом)

- Миграция `make migrate-create name=exercise_favorites` → `00008_exercise_favorites.sql`:

```sql
CREATE TABLE exercise_favorites (
  user_id      INT UNSIGNED NOT NULL,
  exercise_id  INT UNSIGNED NOT NULL,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, exercise_id),
  KEY ix_exercise_favorites_user_created (user_id, created_at),
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
  FOREIGN KEY (exercise_id) REFERENCES exercises (id) ON DELETE CASCADE
);
```

- Бэкенд: `POST /api/v1/exercises/:id/favorite` (идемпотентно, `INSERT IGNORE`), `DELETE /api/v1/exercises/:id/favorite`; `is_favorite` (LEFT JOIN) в списке и деталях; фильтр `favorites_only=1`
- Фронт: сердечко на карточке с optimistic-обновлением (откат + toast при ошибке), чип «Избранное» в панели фильтров
- Тесты: toggle on/off, `favorites_only`, корректность `is_favorite` в списке и деталях

---

## Проверка после каждого этапа

- Бэкенд: из `backend/` — `go build ./... && go vet ./...`; из корня — `make test-db-up && make test`
- Фронтенд: из `frontend/` — `npm run lint && npm run build` (классы Tailwind не сортировать руками — `npm run format`)
- Ручной смоук: `make up` + `make seed-catalog` → страница `/exercises` (фильтры, поиск кириллицей, пагинация, GIF, диалог деталей, attribution)

## Замечания

- Лицензия Gym visual: медиа только 180×180 (без апскейла), attribution обязательна рядом с медиа — в карточке компактно, в диалоге полностью
- FULLTEXT InnoDB обновляется при COMMIT — в тестах фикстуры видны после вставки в завершённой транзакции
- В роутере `GET /exercises/filters` и `GET /exercises/:id`: `filters` не конфликтует (конкретный сегмент против `:id` в Gin), но порядок регистрации — filters раньше `:id`
- Sidebar ведёт на `/exercises` по точному совпадению пути (`route.path === item.to`) — диалог деталей URL не меняет, проблемы нет
