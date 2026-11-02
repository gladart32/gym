<script setup>
import { RouterView, useRoute } from 'vue-router'

import AppSidebar from '@/components/layout/AppSidebar.vue'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'

const route = useRoute()
</script>

<template>
  <SidebarProvider>
    <AppSidebar />
    <SidebarInset>
      <header class="flex h-14 shrink-0 items-center gap-3 border-b px-4">
        <SidebarTrigger />
        <span class="text-sm font-medium">{{ route.meta.title }}</span>
      </header>
      <div class="flex-1 p-6">
        <RouterView />
      </div>
    </SidebarInset>
  </SidebarProvider>
</template>
