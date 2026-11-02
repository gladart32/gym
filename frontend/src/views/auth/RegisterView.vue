<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const { t } = useI18n()

const name = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')

function onSubmit() {
  if (password.value !== confirmPassword.value) {
    toast.error(t('auth.toastPasswordsMismatch'))
    return
  }
  toast.info(t('auth.toastRegisterNotReady'))
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
        <Button type="submit" class="w-full">{{ $t('auth.createAccount') }}</Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.hasAccount') }}
        <RouterLink to="/auth/login" class="underline">{{ $t('common.login') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
