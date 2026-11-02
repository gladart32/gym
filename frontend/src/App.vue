<script setup>
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterView, useRoute } from 'vue-router'

import { Toaster } from '@/components/ui/sonner'

const { t, locale } = useI18n()
const route = useRoute()

watch(locale, () => {
  const title = route.meta.title ? t(route.meta.title) : null
  document.title = title ? `${title} · Gym` : 'Gym'
})
</script>

<template>
  <RouterView />
  <Toaster rich-colors position="top-right" />
</template>
