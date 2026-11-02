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
  <li data-slot="breadcrumb-item" :class="cn('inline-flex items-center gap-1', props.class)">
    <slot />
  </li>
</template>
