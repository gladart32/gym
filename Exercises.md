# Exercises

План реализации раздела «Упражнения» (каталог с фильтрами, поиском, пагинацией и визуализацией). Составлен по итогам исследования бэкенда/фронтенда и схемы БД (`docs/DB.md`).

## Статус реализации

| Этап | Статус |
|---|---|
| 1. Бэкенд каталога | **Реализован** (см. ниже) |
| 2. Фронтенд | **Реализован** (см. ниже) |
| 3. Избранное | **Реализован** (см. ниже) |

Раздел «Упражнения» реализован полностью. Изменения не закоммичены (git-операции — только по явному разрешению пользователя).

### Что реализовано в этапе 3 «Избранное» (10.11.2026)

- Миграция `00008_exercise_favorites.sql` (PK `user_id+exercise_id`, FK CASCADE, индекс `(user_id, created_at)`), применена на dev и тест-базы (`make migrate-up`, `testutil` поднимает сам)
- Бэкенд: `favorites.go` — `AddFavorite` (проверка существования/`is_active` → 404, затем `INSERT IGNORE`) и `RemoveFavorite` (идемпотентный DELETE); `POST`/`DELETE /api/v1/exercises/:id/favorite` отвечают `{is_favorite: bool}`; `is_favorite` через `LEFT JOIN exercise_favorites` в списке и детальной; фильтр `favorites_only=1` в ListFilter (+ LEFT JOIN в count-запросе); в хендлере вынесен `parseExerciseID`
- `registerAndLogin` в тестах возвращает `(token, userID)` — точная проверка счётчика строк
- Фронт: сердечко на `ExerciseCard` (absolute над медиа, `@click.stop`, aria-label меняется на «Убрать из избранного», заливка `fill-destructive`), optimistic-toggle в `ExercisesView` с откатом + toast при ошибке; в виде «Избранное» снятие лайка форсирует перезагрузку списка (сброс `lastParamsKey`); переключатель «Избранное» первым блоком в `ExerciseFiltersPanel`; URL-параметр `fav`; i18n-ключи `favorites`/`addToFavorites`/`removeFromFavorites` в ru/en
- Тесты: `TestFavorites` — 401 без токена, 404 (несуществующее/неактивное), идемпотентность add/remove, `is_favorite` в списке и детальной, `favorites_only`, изоляция пользователей; полный набор `go build/vet`, `GYM_ENV=test go test ./...`, `npm run lint/build` — зелёные
- Смоук (nginx :80, carol@gym.local): лайк/анлайк с оптимистикой, персистентность после F5, фильтр `?fav=1` (2 карточки), снятие лайка в виде «Избранное» перечитывает список, empty state, restore из URL, консоль без ошибок; тестовые лайки удалены
- Нюанс деплоя: при пересборке app-контейнера nginx мог закэшировать старый IP апстрима — `docker restart gym-nginx`

### Текущее состояние (обновлено 10.11.2026, вечер)

**Окружение:** тест-инфраструктура поднята (`make test-db-up`, :3307/:6380); app-контейнер пересобран с кодом этапов 1–3; в `dev` базе есть пользователь `smoke1@gym.test` / `smoke-pass-1` (verified) и `carol@gym.local` (инвентарь: Штанга, Гантели, Гиря); каталог засеян (`make seed-catalog`, 1324 упр.).

### Что реализовано в этапе 1 (10.11.2026)

**Домен `backend/internal/exercises/`:**
- `service.go` — `Service{db, log}` по образцу `internal/profile`
- `lang.go` — `LangFromAcceptLanguage` (ru/en, фолбэк ru) + выбор колонки `name_ru`/`name_en`
- `list.go` — список: динамический WHERE (поиск FULLTEXT ngram `MATCH … AGAINST(BOOLEAN MODE)` с экранированием операторов, фолбэк `LIKE` для запросов короче 2 рун или состоящих только из операторов; `IN(…)` для body_part_ids / equipment_ids / muscle_ids / tracking_types; `my_equipment` через `user_equipment` + всегда `equipment.code='body weight'`), сортировка `name`/`newest` × `asc`/`desc`, LIMIT/OFFSET + отдельный COUNT, нормализация путей медиа (`/media/` + path). Нюанс: `sort=newest` без явного `order` отдаёт новые сверху (DESC)
- `detail.go` — детальная карточка (перевод, steps JSON, instructions, вторичные мышцы, медиа с attribution), доменная ошибка `ErrNotFound` → 404
- `filters.go` — локализованные справочники + `IsTrackingType` (whitelist), метки tracking_type ru/en

**Эндпоинты** (группа за `middleware.Auth`, `internal/transport/http/handler/exercises.go`):
- `GET /api/v1/exercises` — query: `q, body_part_ids, equipment_ids, muscle_ids, tracking_types, my_equipment, sort, order, page, per_page` (дефолт 20, макс 50, лимит 100 id на список; невалидные значения → 400)
- `GET /api/v1/exercises/filters` — справочники по `Accept-Language`
- `GET /api/v1/exercises/:id` — детальная, 404 для неизвестных и `is_active=0`

**Подключение:** `server/router.go` + `server/server.go` (сигнатуры `New`/`NewRouter` расширены), `app.go` (поле `Exercises`, DI), обновлены вызовы в `auth_test.go` и `health_test.go`.

**Тесты** `internal/exercises/exercises_test.go` (8 интеграционных, внеш. пакет, инфраструктура `make test-db-up`): 401 без токена; дефолты/пагинация/сортировки/локализация ru-en; все фильтры + комбинация + 400 на невалидные параметры; поиск FULLTEXT/LIKE/en; `my_equipment` до и после выбора инвентаря через `PUT /profile/equipment`; деталь (steps, медиа-URL, вторичные мышцы) + 404 + скрытие неактивных; справочники ru/en; избранное (toggle, `favorites_only`, `is_favorite`, изоляция юзеров).

**Проверки:** `go build ./...`, `go vet ./...`, `GYM_ENV=test go test ./...` — все зелёные.

### Что реализовано в этапе 2 (10.11.2026)

**Компоненты shadcn-vue доустановлены:** `checkbox, badge, pagination, dialog, select, scroll-area, switch` (кнопка не перезаписана; `@lucide/vue` обновлён 1.66 → 1.70).

**Код:**
- `src/api/exercises.js` — `list(params)`, `get(id)`, `filters()` по образцу `profile.js`
- `src/views/ExercisesView.vue` — полная замена заглушки: тулбар (поиск с debounce 400 мс через `refDebounced`, select сортировки), панель фильтров (десктоп — sticky-колонка `lg:`; мобилка — Sheet слева), сетка карточек `2/3/4` колонки, нумерованная пагинация (reka-ui Pagination с ellipsis), Skeleton-загрузка, empty state с кнопкой сброса, тосты об ошибках
- `src/components/exercises/ExerciseFiltersPanel.vue` — чекбокс-секции: часть тела, оборудование (ScrollArea), мышцы (ScrollArea), тип упражнения, Switch «Мой инвентарь» (показывается только если в профиле выбран инвентарь) + tooltip-подсказка; двусторонняя связь через `defineModel`
- `src/components/exercises/ExerciseCard.vue` — GIF (`loading="lazy"`, 180×180, фолбэк «Нет визуализации» при ошибке), компактная подпись © Gym visual (из `attribution`), бейджи части тела и оборудования
- `src/components/exercises/ExerciseDetailDialog.vue` — DialogScrollContent: GIF + полная attribution, бейджи (включая локализованный тип записи), целевая/вспомогательные мышцы, шаги из `steps` (фолбэк `instructions`), Skeleton при загрузке
- i18n: неймспейс `exercises` переписан в `ru.json`/`en.json` (~25 ключей)

**Поведение:**
- Состояние (поиск, фильтры, сортировка, страница) синхронизировано с URL (`router.replace`, короткие ключи `q/bp/eq/mu/tt/my/sort/page`), восстановление при заходе по ссылке
- Смена фильтров сбрасывает страницу в 1; защита от гонок (seq-guard) и от дублей запросов (params-key guard); при сетевой ошибке кэш ключа сбрасывается — повтор возможен
- «Мой инвентарь» отправляет `my_equipment=1`; `initFromUrl` выполняется в setup до регистрации вотчеров (без лишних запросов при restore из URL)

**Найдено и исправлено при смоук-тесте:** отсутствие вотчера `page → loadList` (пагинация меняла только счётчик) — исправлено; дубли начальных запросов — исправлено key-guard'ом.

**Проверки:** `npm run lint`, `npm run build` — зелёные. Ручной смоук по прод-флоу (nginx :80, свежий dist, каталог 1324 шт.): список/пагинация 67 стр., поиск «жим штанги» (debounce), фильтр «Грудь» (163), комбинация фильтр+поиск (104), «Мой инвентарь» (104 → 65 при инвентаре Штанга/Гантели/Гиря), диалог деталей (шаги, мышцы, attribution), мобильный Sheet с фильтрами, восстановление состояния из URL, GIF отдаются nginx'ом (200).

**Известная особенность dev-режима — исправлена:** Vite-прокси `/media` → app-контейнер отдавал 404 (в образе нет медиа). Добавлен bind mount `./backend/media:/media:ro` в сервис `app` (docker-compose.yml) — теперь Gin раздаёт медиа и из контейнера (:8080), дев-прокси (:5173) и nginx (:80) работают одинаково.

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
| POST | `/api/v1/exercises/:id/favorite` | В избранное (идемпотентно; 404 для несуществующего/неактивного) |
| DELETE | `/api/v1/exercises/:id/favorite` | Из избранного (идемпотентно) |

Query-параметры списка: `q`, `body_part_ids`, `equipment_ids`, `muscle_ids` (comma-separated id), `tracking_types` (comma-separated enum), `my_equipment=1`, `favorites_only=1`, `sort=name|newest`, `order=asc|desc`, `page` (≥1), `per_page` (дефолт 20, макс 50).

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
      "is_favorite": false,
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

## Этап 3 — Избранное (реализован, см. «Что реализовано в этапе 3»)

Миграция `00008_exercise_favorites.sql`:

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

- Бэкенд: `POST /api/v1/exercises/:id/favorite` (идемпотентно, `INSERT IGNORE`), `DELETE /api/v1/exercises/:id/favorite`; `is_favorite` (LEFT JOIN) в списке и деталях; фильтр `favorites_only=1` — реализовано
- Фронт: сердечко на карточке с optimistic-обновлением (откат + toast при ошибке), переключатель «Избранное» в панели фильтров, URL-параметр `fav` — реализовано
- Тесты: toggle on/off, `favorites_only`, корректность `is_favorite` в списке и деталях — реализовано (`TestFavorites`)

---

## План улучшений и правок

Накопитель мелких доработок раздела; новые пункты дописывать сюда, реализованные — помечать.

1. ~~Скролл вверх при клике на пагинацию~~ — **реализовано (10.11.2026)**: `onPageSelected` в `ExercisesView.vue` — `window.scrollTo({top:0, behavior:'smooth'})` только на клик по пагинации (restore `page` из URL скролла не делает)
2. ~~Сворачиваемые группы фильтров~~ — **реализовано (10.11.2026)**: заголовки секций в `ExerciseFiltersPanel.vue` — кнопка `aria-expanded` + `ChevronDown` (rotate-180 при раскрытии), тело через `v-show`; по умолчанию развёрнута только «Часть тела»; состояние локальное, между визитами сбрасывается (переключатели «Избранное»/«Мой инвентарь» не сворачиваются)
3. ~~Tooltip у фильтра «Избранное»~~ — **реализовано (10.11.2026)**: тот же паттерн Info/Tooltip, что у «Мой инвентарь»; i18n `exercises.favoritesHint` (ru: «Только упражнения, отмеченные сердечком»). Заодно убран декоративный Heart из подписи — теперь иконка Info единообразна с «Мой инвентарь»

## Проверка после каждого этапа

- Бэкенд: из `backend/` — `go build ./... && go vet ./...`; из корня — `make test-db-up && make test`
- Фронтенд: из `frontend/` — `npm run lint && npm run build` (классы Tailwind не сортировать руками — `npm run format`)
- Ручной смоук: `make up` + `make seed-catalog` → страница `/exercises` (фильтры, поиск кириллицей, пагинация, GIF, диалог деталей, attribution)

## Замечания

- Лицензия Gym visual: медиа только 180×180 (без апскейла), attribution обязательна рядом с медиа — в карточке компактно, в диалоге полностью
- FULLTEXT InnoDB обновляется при COMMIT — в тестах фикстуры видны после вставки в завершённой транзакции
- В роутере `GET /exercises/filters` и `GET /exercises/:id`: `filters` не конфликтует (конкретный сегмент против `:id` в Gin), но порядок регистрации — filters раньше `:id`
- Sidebar ведёт на `/exercises` по точному совпадению пути (`route.path === item.to`) — диалог деталей URL не меняет, проблемы нет
