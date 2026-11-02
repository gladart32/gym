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
const sent = ref(false)

function onSubmit() {
  sent.value = true
  toast.info(t('auth.toastEmailNotReady'))
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ $t('auth.resetTitle') }}</CardTitle>
      <CardDescription>
        {{ sent ? $t('auth.resetSent') : $t('auth.resetDescription') }}
      </CardDescription>
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
        <Button type="submit" class="w-full">{{ $t('auth.sendLink') }}</Button>
      </form>
      <p class="text-muted-foreground mt-4 text-center text-sm">
        {{ $t('auth.rememberedPassword') }}
        <RouterLink to="/auth/login" class="underline">{{ $t('common.login') }}</RouterLink>
      </p>
    </CardContent>
  </Card>
</template>
