<script setup>
import { onMounted, ref } from 'vue'
import { Monitor, Smartphone } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()

const sessions = ref([])
const sessionsLoading = ref(true)

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordLoading = ref(false)

const dateTimeFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' })

function formatLastUsed(session) {
  return session.last_used_at ? dateTimeFmt.format(new Date(session.last_used_at)) : ''
}

async function loadSessions() {
  sessionsLoading.value = true
  try {
    sessions.value = await authApi.sessions()
  } catch (error) {
    toast.error(error.message)
  } finally {
    sessionsLoading.value = false
  }
}

async function onRevoke(session) {
  try {
    await authApi.revokeSession(session.id)
    toast.success(t('profile.sessionTerminated'))
    await loadSessions()
  } catch (error) {
    toast.error(error.message)
  }
}

async function onRevokeAll() {
  try {
    await authApi.revokeAllSessions()
    toast.success(t('profile.allSessionsTerminated'))
    await loadSessions()
  } catch (error) {
    toast.error(error.message)
  }
}

async function onChangePassword() {
  if (newPassword.value !== confirmPassword.value) {
    toast.error(t('auth.toastPasswordsMismatch'))
    return
  }
  passwordLoading.value = true
  try {
    await authApi.changePassword(currentPassword.value, newPassword.value)
    toast.success(t('profile.passwordChanged'))
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (error) {
    toast.error(error.message)
  } finally {
    passwordLoading.value = false
  }
}

async function onLogout() {
  await auth.logout()
  toast.info(t('auth.loggedOut'))
  router.push('/auth/login')
}

onMounted(loadSessions)
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.profile') }}</h1>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.account') }}</CardTitle>
        <CardDescription>{{ auth.user?.email ?? '' }}</CardDescription>
      </CardHeader>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.security') }}</CardTitle>
        <CardDescription>{{ $t('profile.changePasswordDescription') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="flex max-w-sm flex-col gap-4" @submit.prevent="onChangePassword">
          <div class="grid gap-2">
            <Label for="current-password">{{ $t('profile.currentPassword') }}</Label>
            <Input
              id="current-password"
              v-model="currentPassword"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>
          <div class="grid gap-2">
            <Label for="new-password">{{ $t('profile.newPassword') }}</Label>
            <Input
              id="new-password"
              v-model="newPassword"
              type="password"
              autocomplete="new-password"
              minlength="8"
              required
            />
          </div>
          <div class="grid gap-2">
            <Label for="confirm-password">{{ $t('auth.confirmPassword') }}</Label>
            <Input
              id="confirm-password"
              v-model="confirmPassword"
              type="password"
              autocomplete="new-password"
              minlength="8"
              required
            />
          </div>
          <Button type="submit" class="w-fit" :disabled="passwordLoading">
            {{ passwordLoading ? $t('common.loading') : t('profile.savePassword') }}
          </Button>
        </form>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.sessions') }}</CardTitle>
        <CardDescription>{{ $t('profile.sessionsDescription') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <p v-if="sessionsLoading" class="text-muted-foreground text-sm">
          {{ $t('common.loading') }}
        </p>
        <p v-else-if="sessions.length === 0" class="text-muted-foreground text-sm">—</p>
        <ul v-else class="flex flex-col">
          <li
            v-for="session in sessions"
            :key="session.id"
            class="flex items-center justify-between gap-4 py-3"
          >
            <div class="flex min-w-0 items-center gap-3">
              <component
                :is="session.client_type === 'mobile' ? Smartphone : Monitor"
                class="text-muted-foreground size-5 shrink-0"
              />
              <div class="min-w-0">
                <p class="truncate text-sm font-medium">
                  {{ session.device_name || session.client_type }}
                  <span
                    v-if="session.is_current"
                    class="bg-primary/10 text-primary ml-2 rounded-full px-2 py-0.5 text-xs"
                  >
                    {{ $t('profile.thisDevice') }}
                  </span>
                </p>
                <p class="text-muted-foreground truncate text-xs">
                  {{ [session.os, session.app_version, session.ip].filter(Boolean).join(' · ') }}
                  · {{ $t('profile.lastUsed') }}: {{ formatLastUsed(session) }}
                </p>
              </div>
            </div>
            <Button
              v-if="session.is_current"
              variant="outline"
              size="sm"
              @click="onLogout"
            >
              {{ $t('auth.logout') }}
            </Button>
            <Button v-else variant="outline" size="sm" @click="onRevoke(session)">
              {{ $t('profile.terminate') }}
            </Button>
          </li>
        </ul>
        <template v-if="sessions.length > 1">
          <Separator class="my-2" />
          <Button variant="outline" size="sm" @click="onRevokeAll">
            {{ $t('profile.terminateAll') }}
          </Button>
        </template>
      </CardContent>
    </Card>
  </div>
</template>
