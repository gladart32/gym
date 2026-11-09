<script setup>
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { setLocale, supportedLocales } from '@/i18n'
import { useTheme } from '@/composables/useTheme'

const { locale } = useI18n()
const { theme } = useTheme()

const themeOptions = ['auto', 'light', 'dark']
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.settings') }}</h1>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('settings.appearance') }}</CardTitle>
        <CardDescription>{{ $t('settings.appearanceDescription') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="flex items-center justify-between">
          <span class="text-sm">{{ $t('settings.theme') }}</span>
          <div class="flex gap-2">
            <Button
              v-for="option in themeOptions"
              :key="option"
              :variant="theme === option ? 'default' : 'outline'"
              size="sm"
              @click="theme = option"
            >
              {{ $t(`settings.theme_${option}`) }}
            </Button>
          </div>
        </div>
        <Separator class="my-4" />
        <div class="flex items-center justify-between">
          <span class="text-sm">{{ $t('settings.language') }}</span>
          <div class="flex gap-2">
            <Button
              v-for="loc in supportedLocales"
              :key="loc"
              :variant="locale === loc ? 'default' : 'outline'"
              size="sm"
              @click="setLocale(loc)"
            >
              {{ loc.toUpperCase() }}
            </Button>
          </div>
        </div>
        <Separator class="my-4" />
        <p class="text-muted-foreground text-sm">{{ $t('settings.otherLater') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
