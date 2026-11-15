<script setup>
import { computed, onMounted, ref } from 'vue'
import { CalendarCheck, ChevronLeft, ChevronRight, Plus, Trash2 } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import useScheduleStore from '@/stores/schedule'

const { t } = useI18n()
const router = useRouter()
const store = useScheduleStore()

// Порядок дней недели синхронизирован с canonical-порядком MySQL SET
const WEEKDAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun']

function todayStr() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function shiftDays(dateStr, days) {
  const [y, m, d] = dateStr.split('-').map(Number)
  const date = new Date(y, m - 1, d + days)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const today = todayStr()
const anchor = ref(todayStr())

const weekLabel = computed(() => {
  if (!store.week) return ''
  return t('schedule.weekRange', { from: store.week.start_date, to: store.week.end_date })
})

const plannedCount = computed(() =>
  store.week?.days?.reduce((n, d) => n + d.entries.length, 0) ?? 0,
)

function prevWeek() {
  anchor.value = shiftDays(anchor.value, -7)
  store.loadWeek(anchor.value)
}

function nextWeek() {
  anchor.value = shiftDays(anchor.value, 7)
  store.loadWeek(anchor.value)
}

function goToday() {
  anchor.value = todayStr()
  store.loadWeek('')
}

function dayNumber(date) {
  return Number(date.slice(8))
}

function isToday(date) {
  return date === today
}

// --- Правила расписания активных программ ---

async function saveRule(rule, overrides = {}) {
  const body = {
    frequency: rule.frequency,
    interval_n: Math.max(1, Math.round(Number(rule.interval_n) || 1)),
    weekdays: rule.weekdays ?? [],
    once_date: rule.once_date || null,
    is_enabled: rule.is_enabled,
    ...overrides,
  }
  const ok = await store.updateRule(rule, body)
  if (ok) {
    toast.success(t('schedule.ruleUpdated'))
  }
}

function toggleRuleWeekday(rule, code) {
  const set = new Set(rule.weekdays ?? [])
  if (set.has(code)) {
    set.delete(code)
  } else {
    set.add(code)
  }
  if (set.size === 0) {
    toast.error(t('schedule.selectDayHint'))
    return
  }
  rule.weekdays = WEEKDAYS.filter((w) => set.has(w))
  saveRule(rule)
}

function onRuleModeChange(rule, mode) {
  rule.frequency = mode
  if (mode === 'weekly') {
    rule.weekdays = ['mon']
    rule.once_date = null
    rule.interval_n = Math.max(1, Math.round(Number(rule.interval_n) || 1))
  } else if (mode === 'daily') {
    rule.weekdays = []
    rule.once_date = null
    rule.interval_n = Math.max(1, Math.round(Number(rule.interval_n) || 1))
  } else if (mode === 'once') {
    rule.weekdays = []
    rule.once_date = rule.once_date || todayStr()
  }
  saveRule(rule)
}

function onRuleInterval(rule, value) {
  rule.interval_n = Math.max(1, Math.round(Number(value) || 1))
  saveRule(rule)
}

function onRuleDate(rule, value) {
  rule.once_date = value || null
  if (value) {
    saveRule(rule)
  }
}

function onRuleEnabled(rule, value) {
  rule.is_enabled = value
  saveRule(rule)
}

// --- Деактивация программы ---

const deleteOpen = ref(false)
const deleteTarget = ref(null)

function confirmDeactivate(program) {
  deleteTarget.value = program
  deleteOpen.value = true
}

async function deactivate() {
  const ok = await store.deactivate(deleteTarget.value?.id)
  deleteOpen.value = false
  if (ok) {
    toast.success(t('schedule.deactivated'))
  }
}

onMounted(() => {
  store.load()
  store.loadWeek('')
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-3xl font-bold">{{ $t('nav.calendar') }}</h1>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="icon-sm" :aria-label="$t('schedule.prevWeek')" @click="prevWeek">
          <ChevronLeft />
        </Button>
        <Button variant="outline" size="icon-sm" :aria-label="$t('schedule.nextWeek')" @click="nextWeek">
          <ChevronRight />
        </Button>
        <span v-if="weekLabel" class="text-sm font-medium tabular-nums">{{ weekLabel }}</span>
        <Button variant="ghost" size="sm" @click="goToday">
          {{ $t('schedule.today') }}
        </Button>
      </div>
    </div>

    <Skeleton v-if="store.weekLoading && !store.week" class="h-64 w-full" />

    <template v-else-if="store.week">
      <Card v-if="!store.hasPrograms">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <CalendarCheck class="size-5" />
            {{ $t('schedule.noProgramsTitle') }}
          </CardTitle>
          <CardDescription>{{ $t('schedule.noProgramsHint') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button size="sm" @click="router.push({ name: 'workouts' })">
            <Plus />
            {{ $t('schedule.chooseProgram') }}
          </Button>
        </CardContent>
      </Card>

      <template v-else>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-7">
          <div
            v-for="d in store.week.days"
            :key="d.date"
            class="flex min-h-24 flex-col gap-1.5 rounded-lg border p-2"
            :class="isToday(d.date) ? 'border-primary bg-primary/5' : ''"
          >
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium">{{ $t(`weekdays.${d.weekday}`) }}</span>
              <span class="text-muted-foreground text-xs tabular-nums">
                {{ dayNumber(d.date) }}
                <Badge v-if="isToday(d.date)" variant="secondary" class="ml-1 px-1 py-0 text-[10px]">
                  {{ $t('schedule.today') }}
                </Badge>
              </span>
            </div>
            <div
              v-for="entry in d.entries"
              :key="`${entry.user_program_id}-${entry.program_day_id}`"
              class="bg-primary/10 rounded-md border p-1.5 text-xs"
            >
              <p class="leading-tight font-medium">{{ entry.day_name }}</p>
              <p class="text-muted-foreground mt-0.5 line-clamp-1 leading-tight">
                {{ entry.program_name }}
              </p>
            </div>
          </div>
        </div>

        <p v-if="plannedCount === 0" class="text-muted-foreground text-center text-sm">
          {{ $t('schedule.noPlans') }}
        </p>

        <div class="flex flex-col gap-3">
          <h2 class="pt-2 text-lg font-semibold">{{ $t('schedule.activePrograms') }}</h2>

          <Card v-for="program in store.programs" :key="program.id">
            <CardHeader class="flex flex-row items-start justify-between space-y-0">
              <div>
                <CardTitle class="text-base">{{ program.program_name }}</CardTitle>
                <CardDescription>
                  {{ $t('schedule.startDate') }}: {{ program.start_date }}
                </CardDescription>
              </div>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-destructive"
                :aria-label="$t('schedule.deactivate')"
                @click="confirmDeactivate(program)"
              >
                <Trash2 />
              </Button>
            </CardHeader>
            <CardContent class="flex flex-col gap-2">
              <div
                v-for="rule in program.rules"
                :key="rule.id"
                class="flex flex-wrap items-center gap-2 rounded-lg border p-2"
                :class="rule.is_enabled ? '' : 'opacity-60'"
              >
                <span class="min-w-24 text-sm font-medium">{{ rule.day_name }}</span>

                <Select :model-value="rule.frequency" @update:model-value="onRuleModeChange(rule, $event)">
                  <SelectTrigger class="h-7 w-40 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="weekly">{{ $t('schedule.modeWeekly') }}</SelectItem>
                    <SelectItem value="daily">{{ $t('schedule.modeDaily') }}</SelectItem>
                    <SelectItem value="once">{{ $t('schedule.modeOnce') }}</SelectItem>
                  </SelectContent>
                </Select>

                <template v-if="rule.frequency === 'weekly'">
                  <Button
                    v-for="code in WEEKDAYS"
                    :key="code"
                    type="button"
                    size="sm"
                    class="h-7 px-2 text-xs"
                    :variant="(rule.weekdays ?? []).includes(code) ? 'default' : 'outline'"
                    :aria-pressed="(rule.weekdays ?? []).includes(code)"
                    @click="toggleRuleWeekday(rule, code)"
                  >
                    {{ $t(`weekdays.${code}`) }}
                  </Button>
                  <span class="text-muted-foreground text-xs">{{ $t('schedule.intervalWeeks') }}</span>
                  <Input
                    :model-value="rule.interval_n"
                    type="number"
                    min="1"
                    max="255"
                    class="h-7 w-16"
                    @update:model-value="onRuleInterval(rule, $event)"
                  />
                </template>

                <template v-else-if="rule.frequency === 'daily'">
                  <span class="text-muted-foreground text-xs">{{ $t('schedule.intervalDays') }}</span>
                  <Input
                    :model-value="rule.interval_n"
                    type="number"
                    min="1"
                    max="255"
                    class="h-7 w-16"
                    @update:model-value="onRuleInterval(rule, $event)"
                  />
                </template>

                <template v-else-if="rule.frequency === 'once'">
                  <Input
                    :model-value="rule.once_date ?? ''"
                    type="date"
                    class="h-7 w-40"
                    @update:model-value="onRuleDate(rule, $event)"
                  />
                </template>

                <div class="ml-auto">
                  <Switch
                    :model-value="rule.is_enabled"
                    @update:model-value="onRuleEnabled(rule, $event)"
                  />
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </template>
    </template>

    <Dialog v-model:open="deleteOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ $t('schedule.deactivateConfirmTitle') }}</DialogTitle>
          <DialogDescription>{{ $t('schedule.deactivateConfirmHint') }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="deleteOpen = false">
            {{ $t('programs.cancel') }}
          </Button>
          <Button variant="destructive" @click="deactivate">
            {{ $t('schedule.deactivate') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
