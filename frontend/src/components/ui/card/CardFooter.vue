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
    data-slot="card-footer"
    :class="
      cn(
        'bg-muted/50 flex items-center rounded-b-xl border-t p-4 group-data-[size=sm]/card:p-3',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
