<script setup>
import { RouterLink } from 'vue-router'

import { Button } from '@/components/ui/button'
</script>

<template>
  <div class="flex min-h-[50vh] flex-col items-center justify-center gap-4">
    <h1 class="text-muted-foreground text-6xl font-bold">404</h1>
    <p class="text-muted-foreground">{{ $t('notFound.title') }}</p>
    <Button as-child>
      <RouterLink to="/">{{ $t('notFound.home') }}</RouterLink>
    </Button>
  </div>
</template>
