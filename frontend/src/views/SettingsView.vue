<script setup>
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { setLocale, supportedLocales } from '@/i18n'
import { useTheme } from '@/composables/useTheme'
import { useProfileStore } from '@/stores/profile'
import { browserTimezone, POPULAR_TIMEZONES, timezoneOffsetLabel } from '@/utils/date'

const { t, locale } = useI18n()
const { theme } = useTheme()

const themeOptions = ['auto', 'light', 'dark']

// --- Часовой пояс ---

const profileStore = useProfileStore()
const { profile } = storeToRefs(profileStore)

const timezone = ref('')
const timezoneOptions = ref([...POPULAR_TIMEZONES])
const timezoneSaving = ref(false)

const timezoneOffset = computed(() => timezoneOffsetLabel(timezone.value))
const timezoneDirty = computed(() => timezone.value !== (profile.value?.timezone ?? ''))

function ensureTimezoneOption(zone) {
  if (zone && !timezoneOptions.value.includes(zone)) {
    timezoneOptions.value.push(zone)
  }
}

function detectTimezone() {
  const zone = browserTimezone()
  if (!zone) {
    toast.error(t('settings.timezoneDetectFailed'))
    return
  }
  ensureTimezoneOption(zone)
  timezone.value = zone
}

async function saveTimezone() {
  if (!timezone.value) return
  timezoneSaving.value = true
  const p = profile.value ?? {}
  const ok = await profileStore.saveProfile({
    gender: p.gender ?? null,
    birth_date: p.birth_date ? String(p.birth_date).slice(0, 10) : null,
    target_weight_kg: p.target_weight_kg ?? null,
    fitness_level: p.fitness_level ?? null,
    timezone: timezone.value,
  })
  timezoneSaving.value = false
  if (ok) {
    toast.success(t('settings.timezoneSaved'))
  }
}

onMounted(async () => {
  await profileStore.loadProfile()
  const current = profile.value?.timezone || browserTimezone() || 'UTC'
  ensureTimezoneOption(current)
  timezone.value = current
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold text-center sm:text-left">{{ $t('nav.settings') }}</h1>

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
        <div class="flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <span class="text-sm">{{ $t('settings.timezone') }}</span>
            <span v-if="timezoneOffset" class="text-muted-foreground text-xs tabular-nums">
              {{ timezoneOffset }}
            </span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <Select v-model="timezone">
              <SelectTrigger class="w-64">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="zone in timezoneOptions" :key="zone" :value="zone">
                  {{ zone.replaceAll('_', ' ') }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Button variant="outline" size="sm" @click="detectTimezone">
              {{ $t('settings.detectTimezone') }}
            </Button>
            <Button
              size="sm"
              :disabled="timezoneSaving || !timezoneDirty"
              @click="saveTimezone"
            >
              {{ $t('common.save') }}
            </Button>
          </div>
          <p class="text-muted-foreground text-sm">{{ $t('settings.timezoneHint') }}</p>
        </div>
        <Separator class="my-4" />
        <p class="text-muted-foreground text-sm">{{ $t('settings.otherLater') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
