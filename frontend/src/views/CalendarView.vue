<script setup>
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.calendar') }}</h1>
    <Card>
      <CardHeader>
        <CardTitle>{{ $t('common.inDevelopment') }}</CardTitle>
        <CardDescription>{{ $t('calendar.description') }}</CardDescription>
      </CardHeader>
    </Card>
  </div>
</template>
