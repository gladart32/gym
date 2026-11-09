<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input, PasswordInput } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const { t } = useI18n()
const router = useRouter()

const name = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)

async function onSubmit() {
  if (password.value !== confirmPassword.value) {
    toast.error(t('auth.toastPasswordsMismatch'))
    return
  }
  loading.value = true
  try {
    await authApi.register({ email: email.value, password: password.value, name: name.value })
    toast.success(t('auth.registerSuccess'))
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
      <CardTitle>{{ $t('auth.registerTitle') }}</CardTitle>
      <CardDescription>{{ $t('auth.registerDescription') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="flex flex-col gap-4" @submit.prevent="onSubmit">
        <div class="grid gap-2">
          <Label for="name">{{ $t('common.name') }}</Label>
          <Input
            id="name"
            v-model="name"
            type="text"
            :placeholder="$t('common.namePlaceholder')"
            autocomplete="name"
            required
          />
        </div>
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
        <div class="grid gap-2">
          <Label for="password">{{ $t('common.password') }}</Label>
          <PasswordInput
            id="password"
            v-model="password"
            autocomplete="new-password"
            minlength="8"
            required
          />
        </div>
        <div class="grid gap-2">
          <Label for="confirm-password">{{ $t('auth.confirmPassword') }}</Label>
          <PasswordInput
            id="confirm-password"
            v-model="confirmPassword"
            autocomplete="new-password"
            minlength="8"
            required
          />
        </div>
        <Button type="submit" class="w-full" :disabled="loading">
          {{ loading ? $t('common.loading') : $t('auth.createAccount') }}
        </Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.hasAccount') }}
        <RouterLink to="/auth/login" class="underline">{{ $t('common.login') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
