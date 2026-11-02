<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()

const stats = computed(() => [
  { title: t('dashboard.workouts'), value: '—', description: t('dashboard.workoutsHint') },
  { title: t('dashboard.exercises'), value: '—', description: t('dashboard.exercisesHint') },
  { title: t('dashboard.tonnage'), value: '—', description: t('dashboard.tonnageHint') },
])
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.dashboard') }}</h1>
    <div class="grid gap-4 sm:grid-cols-3">
      <Card v-for="stat in stats" :key="stat.title">
        <CardHeader>
          <CardDescription>{{ stat.title }}</CardDescription>
          <CardTitle class="text-4xl">{{ stat.value }}</CardTitle>
        </CardHeader>
        <CardContent class="text-muted-foreground text-sm">
          {{ stat.description }}
        </CardContent>
      </Card>
    </div>
  </div>
</template>
