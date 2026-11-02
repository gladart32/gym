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
  <ol
    data-slot="breadcrumb-list"
    :class="
      cn(
        'text-muted-foreground flex flex-wrap items-center gap-1.5 text-sm wrap-break-word',
        props.class,
      )
    "
  >
    <slot />
  </ol>
</template>
