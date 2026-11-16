<script setup>
// Полноэкранный layout проведения тренировки: без сайдбара и хедера —
// фокус на текущем упражнении и таймерах, удобно на телефоне в зале.
import { RouterView } from 'vue-router'
</script>

<template>
  <div class="bg-background text-foreground min-h-dvh">
    <RouterView />
  </div>
</template>
