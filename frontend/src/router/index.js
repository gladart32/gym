import { createRouter, createWebHistory } from 'vue-router'

import { i18n } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: () => import('@/layouts/DefaultLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
          meta: { title: 'nav.dashboard' },
        },
        {
          path: 'calendar',
          name: 'calendar',
          component: () => import('@/views/CalendarView.vue'),
          meta: { title: 'nav.calendar' },
        },
        {
          path: 'workouts',
          name: 'workouts',
          component: () => import('@/views/WorkoutsView.vue'),
          meta: { title: 'nav.workouts' },
        },
        {
          path: 'workouts/new',
          name: 'program-create',
          component: () => import('@/views/ProgramEditorView.vue'),
          meta: { title: 'programs.createTitle' },
        },
        {
          path: 'workouts/:id/edit',
          name: 'program-edit',
          component: () => import('@/views/ProgramEditorView.vue'),
          meta: { title: 'programs.editTitle' },
        },
        {
          path: 'workouts/:id',
          name: 'program-detail',
          component: () => import('@/views/ProgramDetailView.vue'),
          meta: { title: 'programs.detailTitle' },
        },
        {
          path: 'exercises',
          name: 'exercises',
          component: () => import('@/views/ExercisesView.vue'),
          meta: { title: 'nav.exercises' },
        },
        {
          path: 'analytics',
          name: 'analytics',
          component: () => import('@/views/AnalyticsView.vue'),
          meta: { title: 'nav.analytics' },
        },
        {
          path: 'profile',
          name: 'profile',
          component: () => import('@/views/ProfileView.vue'),
          meta: { title: 'nav.profile' },
        },
        {
          path: 'profile/measurements',
          name: 'measurements',
          component: () => import('@/views/MeasurementsView.vue'),
          meta: { title: 'profile.measurements' },
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/SettingsView.vue'),
          meta: { title: 'nav.settings' },
        },
        {
          path: ':pathMatch(.*)*',
          name: 'not-found',
          component: () => import('@/views/NotFoundView.vue'),
          meta: { title: 'notFound.title' },
        },
      ],
    },
    {
      path: '/auth',
      component: () => import('@/layouts/AuthLayout.vue'),
      redirect: '/auth/login',
      children: [
        {
          path: 'login',
          name: 'login',
          component: () => import('@/views/auth/LoginView.vue'),
          meta: { title: 'auth.loginTitle', guestOnly: true },
        },
        {
          path: 'register',
          name: 'register',
          component: () => import('@/views/auth/RegisterView.vue'),
          meta: { title: 'auth.registerTitle', guestOnly: true },
        },
        {
          path: 'reset-password',
          name: 'reset-password',
          component: () => import('@/views/auth/ResetPasswordView.vue'),
          meta: { title: 'auth.resetTitle', guestOnly: true },
        },
        {
          path: 'confirm-email',
          name: 'confirm-email',
          component: () => import('@/views/auth/ConfirmEmailView.vue'),
          meta: { title: 'auth.confirmTitle' },
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.boot()
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { path: '/' }
  }
})

router.afterEach((to) => {
  const title = to.meta.title ? i18n.global.t(to.meta.title) : null
  document.title = title ? `${title} · Gym` : 'Gym'
})

export default router
