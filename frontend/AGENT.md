# AGENT.md

Фронтенд проекта Gym. Подробный контекст для AI-агента.

## Стек

- Vue 3 (JavaScript) + Vite, Vue Router, Pinia, VueUse
- UI: Tailwind CSS v4 + shadcn-vue (reka-ui), иконки `@lucide/vue`
- i18n: vue-i18n (`ru` + `en`)
- HTTP: axios · Тосты: vue-sonner · Форматирование: Prettier + prettier-plugin-tailwindcss
- Мобильное приложение — позже через Capacitor (SPA оборачивается в нативную оболочку)

## Команды

```bash
npm run dev     # :5173, прокси /api → localhost:8080
npm run build   # production-сборка в dist/
npm run lint    # oxlint + eslint (с --fix)
npm run format  # Prettier + сортировка Tailwind-классов
```

## Структура

```
src/
  api/http.js            — axios-клиент: base /api/v1, шлёт Accept-Language, интерцептор вытаскивает message из error-envelope
  i18n/index.js          — vue-i18n: ru+en, локаль в localStorage (gym-locale), дефолт — язык браузера, фолбэк ru; смена только через setLocale()
  locales/{ru,en}.json   — переводы UI, неймспейсы по фичам (nav, auth, dashboard, ...)
  layouts/               — DefaultLayout (сайдбар + контент) и AuthLayout (центрированная карточка для auth-страниц)
  components/layout/     — AppSidebar (меню, сворачивается до иконок, на мобилке — Sheet)
  components/ui/         — shadcn-vue компоненты (наш код, можно править)
  composables/useTheme.js — тёмная тема: VueUse useDark (localStorage, по умолчанию системная)
  views/                 — DashboardView (/), WorkoutsView, ExercisesView, AnalyticsView, SettingsView, NotFoundView
  views/auth/            — LoginView, RegisterView, ResetPasswordView, ConfirmEmailView (читает ?token=)
```

Роуты: `/` Главная, `/workouts`, `/exercises`, `/analytics`, `/settings`, 404 catch-all; auth: `/auth/login`, `/auth/register`, `/auth/reset-password`, `/auth/confirm-email`. `meta.title` хранит **ключ перевода**, рендерится в DefaultLayout и afterEach (document.title обновляется и при смене языка — watch в App.vue).

## i18n — где чьи переводы

- **Строки UI** (кнопки, меню, подсказки, валидация форм) — только в `src/locales/*.json`. Никаких хардкодов в шаблонах
- **Сущности и ошибки API** (упражнения, категории, message от сервера) — локализует бэкенд по `Accept-Language` (интерцептор http.js шлёт автоматически). На фронте не дублируются
- Смена языка: `setLocale()` из `@/i18n` (обновляет i18n, localStorage и `lang` на `<html>`)
- Переключатель языка: Настройки; `supportedLocales` — источник списка

## Паттерны

- UI-компоненты добавлять: `npx shadcn-vue@latest add <name>` (существующие перезаписывать флагом `-o`)
- Auth-формы — заглушки (сабмит → тост): эндпоинтов на бэкенде пока нет, при подключении API использовать `@/api/http.js`
- Тосты: `import { toast } from 'vue-sonner'`; `<Toaster rich-colors position="top-right" />` уже в App.vue
- Тёмная тема: класс `.dark` на `<html>` (уже учтён в CSS-переменных main.css), переключение только через `useTheme`
- Tailwind-классы не сортировать руками — `npm run format` делает это сам

## Известные грабли

- `vue/multi-word-component-names` отключён только для `src/components/ui/**` (eslint.config.js)
- axios-интерцептор реджектит `Error` с message из envelope — ловить через `try/catch`, а не читать `error.response`
