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
    data-slot="card-title"
    :class="
      cn(
        'cn-font-heading text-base leading-snug font-medium group-data-[size=sm]/card:text-sm',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
