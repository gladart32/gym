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

const email = ref('')
const password = ref('')

function onSubmit() {
  toast.info(t('auth.toastLoginNotReady'))
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ $t('auth.loginTitle') }}</CardTitle>
      <CardDescription>{{ $t('auth.loginDescription') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="flex flex-col gap-4" @submit.prevent="onSubmit">
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
          <div class="flex items-center justify-between">
            <Label for="password">{{ $t('common.password') }}</Label>
            <RouterLink
              to="/auth/reset-password"
              class="text-muted-foreground text-sm hover:underline"
            >
              {{ $t('auth.forgotPassword') }}
            </RouterLink>
          </div>
          <Input
            id="password"
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
          />
        </div>
        <Button type="submit" class="w-full">{{ $t('common.login') }}</Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.noAccount') }}
        <RouterLink to="/auth/register" class="underline">{{ $t('common.register') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
