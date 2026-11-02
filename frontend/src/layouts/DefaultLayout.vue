<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterView, useRoute } from 'vue-router'

import AppSidebar from '@/components/layout/AppSidebar.vue'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'

const { t } = useI18n()
const route = useRoute()

const title = computed(() => (route.meta.title ? t(route.meta.title) : ''))
</script>

<template>
  <SidebarProvider>
    <AppSidebar />
    <SidebarInset>
      <header class="flex h-14 shrink-0 items-center gap-3 border-b px-4">
        <SidebarTrigger />
        <span class="text-sm font-medium">{{ title }}</span>
      </header>
      <div class="flex-1 p-6">
        <RouterView />
      </div>
    </SidebarInset>
  </SidebarProvider>
</template>
