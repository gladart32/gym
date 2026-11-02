<script setup>
import { Dumbbell } from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="bg-muted flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
    <div class="flex w-full max-w-sm flex-col gap-6">
      <RouterLink to="/" class="flex items-center gap-2 self-center font-medium">
        <div
          class="bg-primary text-primary-foreground flex size-8 items-center justify-center rounded-lg"
        >
          <Dumbbell class="size-4" />
        </div>
        Gym
      </RouterLink>
      <RouterView />
    </div>
  </div>
</template>
