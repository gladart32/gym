<script setup>
import { MailCheck } from '@lucide/vue'
import { RouterLink, useRoute } from 'vue-router'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const route = useRoute()
const token = route.query.token
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
        <template v-if="token">{{ $t('auth.confirmPending') }}</template>
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
