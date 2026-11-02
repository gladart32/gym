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
    data-slot="avatar-group"
    :class="
      cn(
        'group/avatar-group *:data-[slot=avatar]:ring-background flex -space-x-2 *:data-[slot=avatar]:ring-2',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
