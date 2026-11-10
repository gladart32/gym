<script setup>
import { cn } from 'cn'

const props = defineProps({
  class: {
    type: [Boolean, null, String, Object, Array],
    required: false,
    skipCheck: true,
  },
})
</script>

<template>
  <div data-slot="dialog-header" :class="cn('flex flex-col gap-2', props.class)">
    <slot />
  </div>
</template>
