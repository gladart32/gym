<script setup>
import { MoreHorizontalIcon } from '@lucide/vue'
import { reactiveOmit } from '@vueuse/core'
import { cn } from 'cn'
import { PaginationEllipsis } from 'reka-ui'

const props = defineProps({
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
  <PaginationEllipsis
    data-slot="pagination-ellipsis"
    v-bind="delegatedProps"
    :class="
      cn('flex size-8 items-center justify-center [&_svg:not([class*=size-])]:size-4', props.class)
    "
  >
    <slot>
      <MoreHorizontalIcon />
      <span class="sr-only">More pages</span>
    </slot>
  </PaginationEllipsis>
</template>
