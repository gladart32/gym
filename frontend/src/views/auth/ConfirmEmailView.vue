<script setup>
import { computed, onMounted, ref } from 'vue'
import { MailCheck } from '@lucide/vue'
import { RouterLink, useRoute } from 'vue-router'

import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const route = useRoute()

const token = computed(() => {
  const value = route.query.token
  return typeof value === 'string' && value.length > 0 ? value : null
})

// pending | success | invalid
const state = ref('pending')

onMounted(async () => {
  if (!token.value) {
    state.value = 'invalid'
    return
  }
  try {
    await authApi.verifyEmail(token.value)
    state.value = 'success'
  } catch {
    state.value = 'invalid'
  }
})
</script>

<template>
  <Card>
    <CardHeader class="items-center text-center">
      <div
        class="bg-primary/10 text-primary mx-auto mb-2 flex size-12 items-center justify-center rounded-full"
      >
        <MailCheck class="size-6" />
      </div>
      <CardTitle>{{ $t('auth.confirmTitle') }}</CardTitle>
      <CardDescription>
        <template v-if="state === 'pending'">{{ $t('auth.confirmPending') }}</template>
        <template v-else-if="state === 'success'">{{ $t('auth.confirmSuccess') }}</template>
        <template v-else>{{ $t('auth.confirmInvalid') }}</template>
      </CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-3">
      <Button as-child class="w-full">
        <RouterLink to="/auth/login">{{ $t('auth.goToLogin') }}</RouterLink>
      </Button>
    </CardContent>
  </Card>
</template>
