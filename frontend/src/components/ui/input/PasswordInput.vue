<script setup>
import { ref } from 'vue'
import { useVModel } from '@vueuse/core'
import { Eye, EyeOff } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { cn } from 'cn'

import Input from './Input.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps({
  defaultValue: { type: [String, Number], required: false },
  modelValue: { type: [String, Number], required: false },
  class: {
    type: [Boolean, null, String, Object, Array],
    required: false,
    skipCheck: true,
  },
})

const emits = defineEmits(['update:modelValue'])

const { t } = useI18n()

const modelValue = useVModel(props, 'modelValue', emits, {
  passive: true,
  defaultValue: props.defaultValue,
})

const show = ref(false)
</script>

<template>
  <div class="relative">
    <Input
      v-model="modelValue"
      v-bind="$attrs"
      :type="show ? 'text' : 'password'"
      :class="cn('pr-9', props.class)"
    />
    <button
      type="button"
      class="text-muted-foreground hover:text-foreground absolute inset-y-0 right-0 flex w-9 cursor-pointer items-center justify-center outline-none"
      :aria-label="show ? t('common.hidePassword') : t('common.showPassword')"
      :aria-pressed="show"
      @click="show = !show"
    >
      <EyeOff v-if="show" class="size-4" />
      <Eye v-else class="size-4" />
    </button>
  </div>
</template>
