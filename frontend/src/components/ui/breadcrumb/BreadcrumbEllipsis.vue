<script setup>
import { MoreHorizontalIcon } from '@lucide/vue'
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
  <span
    data-slot="breadcrumb-ellipsis"
    role="presentation"
    aria-hidden="true"
    :class="cn('flex size-5 items-center justify-center [&>svg]:size-4', props.class)"
  >
    <slot>
      <MoreHorizontalIcon />
    </slot>
    <span class="sr-only">More</span>
  </span>
</template>
