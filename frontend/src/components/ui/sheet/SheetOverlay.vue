<script setup>
import { reactiveOmit } from '@vueuse/core'
import { cn } from 'cn'
import { DialogOverlay } from 'reka-ui'

const props = defineProps({
  forceMount: { type: Boolean, required: false },
  asChild: { type: Boolean, required: false },
  as: { type: null, required: false },
  class: {
    type: [Boolean, null, String, Object, Array],
    required: false,
    skipCheck: true,
  },
})

const delegatedProps = reactiveOmit(props, 'class')
</script>

<template>
  <DialogOverlay
    data-slot="sheet-overlay"
    :class="
      cn(
        'data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0 fixed inset-0 z-50 bg-black/10 duration-100 supports-backdrop-filter:backdrop-blur-xs',
        props.class,
      )
    "
    v-bind="delegatedProps"
  >
    <slot />
  </DialogOverlay>
</template>
