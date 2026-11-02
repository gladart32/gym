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
  <div
    data-slot="sidebar-group-content"
    data-sidebar="group-content"
    :class="cn('w-full text-sm', props.class)"
  >
    <slot />
  </div>
</template>
