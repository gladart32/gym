<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const token = computed(() => {
  const value = route.query.token
  return typeof value === 'string' && value.length > 0 ? value : null
})

const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const sent = ref(false)
const loading = ref(false)

async function onRequestReset() {
  loading.value = true
  try {
    await authApi.forgotPassword(email.value)
    sent.value = true
    toast.success(t('auth.resetSent'))
  } catch (error) {
    toast.error(error.message)
  } finally {
    loading.value = false
  }
}

async function onResetPassword() {
  if (password.value !== confirmPassword.value) {
    toast.error(t('auth.toastPasswordsMismatch'))
    return
  }
  loading.value = true
  try {
    await authApi.resetPassword(token.value, password.value)
    toast.success(t('auth.resetSuccess'))
    router.push('/auth/login')
  } catch (error) {
    toast.error(error.message)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ $t('auth.resetTitle') }}</CardTitle>
      <CardDescription>
        {{ token ? $t('auth.resetNewPasswordDescription') : $t('auth.resetDescription') }}
      </CardDescription>
    </CardHeader>
    <CardContent>
      <form v-if="!token" class="flex flex-col gap-4" @submit.prevent="onRequestReset">
        <div class="grid gap-2">
          <Label for="email">{{ $t('common.email') }}</Label>
          <Input
            id="email"
            v-model="email"
            type="email"
            :placeholder="$t('common.emailPlaceholder')"
            autocomplete="email"
            required
          />
        </div>
        <Button type="submit" class="w-full" :disabled="loading || sent">
          {{ loading ? $t('common.loading') : $t('auth.sendLink') }}
        </Button>
      </form>
      <form v-else class="flex flex-col gap-4" @submit.prevent="onResetPassword">
        <div class="grid gap-2">
          <Label for="password">{{ $t('auth.newPassword') }}</Label>
          <Input
            id="password"
            v-model="password"
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
        <Button type="submit" class="w-full" :disabled="loading">
          {{ loading ? $t('common.loading') : $t('common.save') }}
        </Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.rememberedPassword') }}
        <RouterLink to="/auth/login" class="underline">{{ $t('common.login') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
