import { createRouter, createWebHistory } from 'vue-router'

import { i18n } from '@/i18n'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: () => import('@/layouts/DefaultLayout.vue'),
      children: [
        {
          path: '',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
          meta: { title: 'nav.dashboard' },
        },
        {
          path: 'workouts',
          name: 'workouts',
          component: () => import('@/views/WorkoutsView.vue'),
          meta: { title: 'nav.workouts' },
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
          meta: { title: 'auth.loginTitle' },
        },
        {
          path: 'register',
          name: 'register',
          component: () => import('@/views/auth/RegisterView.vue'),
          meta: { title: 'auth.registerTitle' },
        },
        {
          path: 'reset-password',
          name: 'reset-password',
          component: () => import('@/views/auth/ResetPasswordView.vue'),
          meta: { title: 'auth.resetTitle' },
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

router.afterEach((to) => {
  const title = to.meta.title ? i18n.global.t(to.meta.title) : null
  document.title = title ? `${title} · Gym` : 'Gym'
})

export default router
