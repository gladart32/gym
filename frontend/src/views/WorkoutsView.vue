<script setup>
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.workouts') }}</h1>
    <Card>
      <CardHeader>
        <CardTitle>{{ $t('common.inDevelopment') }}</CardTitle>
        <CardDescription>{{ $t('workouts.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button disabled>{{ $t('workouts.start') }}</Button>
      </CardContent>
    </Card>
  </div>
</template>
