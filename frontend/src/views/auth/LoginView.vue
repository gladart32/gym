<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const loading = ref(false)

async function onSubmit() {
  loading.value = true
  try {
    await auth.login(email.value, password.value)
    const redirect = route.query.redirect
    const target =
      typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//')
        ? redirect
        : '/'
    router.push(target)
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
        <Button type="submit" class="w-full" :disabled="loading">
          {{ loading ? $t('common.loading') : $t('common.login') }}
        </Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.noAccount') }}
        <RouterLink to="/auth/register" class="underline">{{ $t('common.register') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
