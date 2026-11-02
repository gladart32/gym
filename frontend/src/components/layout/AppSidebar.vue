<script setup>
import { computed } from 'vue'
import {
  ChartColumn,
  Dumbbell,
  LayoutDashboard,
  ListChecks,
  Moon,
  Settings,
  Sun,
} from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()
const route = useRoute()
const { isDark, toggleTheme } = useTheme()

const items = computed(() => [
  { title: t('nav.dashboard'), to: '/', icon: LayoutDashboard },
  { title: t('nav.workouts'), to: '/workouts', icon: Dumbbell },
  { title: t('nav.exercises'), to: '/exercises', icon: ListChecks },
  { title: t('nav.analytics'), to: '/analytics', icon: ChartColumn },
  { title: t('nav.settings'), to: '/settings', icon: Settings },
])
</script>

<template>
  <Sidebar collapsible="icon">
    <SidebarHeader>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton size="lg" as-child>
            <RouterLink to="/">
              <div
                class="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg"
              >
                <Dumbbell class="size-4" />
              </div>
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">Gym</span>
                <span class="text-muted-foreground truncate text-xs">{{ $t('nav.tagline') }}</span>
              </div>
            </RouterLink>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarHeader>
    <SidebarContent>
      <SidebarGroup>
        <SidebarGroupLabel>{{ $t('nav.menu') }}</SidebarGroupLabel>
        <SidebarGroupContent>
          <SidebarMenu>
            <SidebarMenuItem v-for="item in items" :key="item.to">
              <SidebarMenuButton as-child :tooltip="item.title">
                <RouterLink :to="item.to" :data-active="route.path === item.to">
                  <component :is="item.icon" />
                  <span>{{ item.title }}</span>
                </RouterLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroupContent>
      </SidebarGroup>
    </SidebarContent>
    <SidebarFooter>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton :tooltip="$t('nav.toggleTheme')" @click="toggleTheme">
            <Sun v-if="isDark" />
            <Moon v-else />
            <span>{{ $t('nav.toggleTheme') }}</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
    </SidebarFooter>
  </Sidebar>
</template>
