<script setup>
// Шапка сессии: название тренировки, общий таймер, переключатель
// режима (ручной/авто) и выход (завершить/отменить).
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { House, Timer } from '@lucide/vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { formatClock } from '@/utils/format'
import useWorkoutSessionStore from '@/stores/workoutSession'

const { t } = useI18n()
const router = useRouter()
const store = useWorkoutSessionStore()

const exitOpen = ref(false)

const elapsed = computed(() => formatClock(store.totalSeconds))

function toggleMode(value) {
  store.mode = value ? 'auto' : 'manual'
  // перешли в авто прямо во время ожидания — сразу запускаем подход
  if (value && store.phase === 'workout' && store.activity === 'idle') {
    store.beginSet()
  }
}

async function finish() {
  exitOpen.value = false
  const ok = await store.complete()
  if (!ok) return
  toast.success(t('session.toast.saved'))
  store.reset()
  router.replace({ name: 'dashboard' })
}

async function cancelWorkout() {
  exitOpen.value = false
  const ok = await store.cancel()
  if (!ok) return
  toast.success(t('session.toast.cancelled'))
  store.reset()
  router.replace({ name: 'dashboard' })
}
</script>

<template>
  <header class="bg-background/95 supports-[backdrop-filter]:bg-background/80 sticky top-0 z-20 border-b backdrop-blur">
    <div class="mx-auto flex w-full max-w-3xl items-center gap-3 px-4 py-3">
      <div class="flex min-w-0 flex-1 flex-col">
        <p class="truncate text-sm font-semibold leading-tight">
          {{ store.session?.name ?? $t('session.title') }}
        </p>
        <p class="text-muted-foreground flex items-center gap-1.5 text-xs tabular-nums">
          <Timer class="size-3.5" />
          {{ elapsed }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <Badge
          :variant="store.mode === 'auto' ? 'default' : 'outline'"
          class="cursor-pointer select-none"
          role="button"
          tabindex="0"
          :title="store.mode === 'auto' ? $t('session.mode.autoHint') : $t('session.mode.manualHint')"
          @click="toggleMode(store.mode !== 'auto')"
          @keydown.enter="toggleMode(store.mode !== 'auto')"
        >
          {{ store.mode === 'auto' ? $t('session.mode.auto') : $t('session.mode.manual') }}
        </Badge>
        <Button variant="ghost" size="icon" :aria-label="$t('session.workout.exit')" @click="exitOpen = true">
          <House />
        </Button>
      </div>
    </div>
    <div class="mx-auto flex w-full max-w-3xl items-center gap-2 px-4 pb-2">
      <Label for="session-mode" class="text-muted-foreground text-xs">
        {{ $t('session.mode.manual') }}
      </Label>
      <Switch
        id="session-mode"
        :model-value="store.mode === 'auto'"
        @update:model-value="toggleMode"
      />
      <Label for="session-mode" class="text-muted-foreground text-xs">
        {{ $t('session.mode.auto') }}
      </Label>
    </div>
  </header>

  <Dialog v-model:open="exitOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ $t('session.workout.exit') }}</DialogTitle>
        <DialogDescription>{{ $t('session.workout.completeConfirm') }}</DialogDescription>
      </DialogHeader>
      <DialogFooter class="flex flex-col gap-2 sm:flex-row">
        <Button variant="outline" class="w-full sm:w-auto" @click="exitOpen = false">
          {{ $t('common.back') }}
        </Button>
        <Button
          variant="outline"
          class="text-destructive w-full sm:w-auto"
          :disabled="store.saving"
          @click="cancelWorkout"
        >
          {{ $t('session.workout.cancel') }}
        </Button>
        <Button class="w-full sm:w-auto" :disabled="store.saving" @click="finish">
          {{ $t('session.workout.complete') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
