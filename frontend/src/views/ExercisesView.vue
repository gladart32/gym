<script setup>
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.exercises') }}</h1>
    <Card>
      <CardHeader>
        <CardTitle>{{ $t('common.inDevelopment') }}</CardTitle>
        <CardDescription>{{ $t('exercises.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button disabled>{{ $t('exercises.add') }}</Button>
      </CardContent>
    </Card>
  </div>
</template>
