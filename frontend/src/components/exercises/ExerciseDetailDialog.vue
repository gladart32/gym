<script setup>
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import { exercisesApi } from '@/api/exercises'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogDescription,
  DialogHeader,
  DialogScrollContent,
  DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'

const props = defineProps({
  exerciseId: { type: Number, default: null },
  trackingTypeNames: { type: Object, default: () => ({}) },
})

const open = defineModel('open', { type: Boolean, default: false })

const { t } = useI18n()
const detail = ref(null)
const loading = ref(false)
const mediaError = ref(false)

watch(open, async (value) => {
  if (!value || !props.exerciseId) return
  loading.value = true
  mediaError.value = false
  detail.value = null
  try {
    detail.value = await exercisesApi.get(props.exerciseId)
  } catch (error) {
    toast.error(error.message)
    open.value = false
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogScrollContent class="sm:max-w-2xl">
      <div v-if="loading" class="flex flex-col gap-4 py-4">
        <div class="flex gap-4">
          <Skeleton class="size-40 shrink-0 rounded-md" />
          <div class="flex w-full flex-col gap-2">
            <Skeleton class="h-6 w-3/4" />
            <Skeleton class="h-4 w-1/2" />
            <Skeleton class="h-4 w-2/3" />
          </div>
        </div>
        <Skeleton class="h-24 w-full" />
      </div>

      <template v-else-if="detail">
        <DialogHeader>
          <DialogTitle>{{ detail.name }}</DialogTitle>
          <DialogDescription>
            {{ detail.body_part.name }} · {{ detail.equipment.name }}
          </DialogDescription>
        </DialogHeader>

        <div class="flex flex-col gap-4">
          <div class="flex flex-col gap-4 sm:flex-row">
            <div
              class="bg-muted relative flex aspect-square size-40 shrink-0 items-center justify-center self-center overflow-hidden rounded-md sm:self-start"
            >
              <img
                v-if="!mediaError && detail.media?.length"
                :src="detail.media.find((m) => m.type === 'gif')?.url ?? detail.media[0].url"
                :alt="detail.name"
                class="size-full object-contain"
                @error="mediaError = true"
              />
              <span v-else class="text-muted-foreground px-4 text-center text-xs">
                {{ t('exercises.noMedia') }}
              </span>
              <span
                v-if="!mediaError && detail.media?.length && detail.media[0].attribution"
                class="bg-background/70 text-muted-foreground absolute right-1 bottom-1 rounded px-1 text-[10px] leading-4"
              >
                {{ detail.media[0].attribution }}
              </span>
            </div>

            <div class="flex min-w-0 flex-1 flex-col gap-3">
              <div class="flex flex-wrap gap-1">
                <Badge variant="secondary">{{ detail.body_part.name }}</Badge>
                <Badge variant="outline">{{ detail.equipment.name }}</Badge>
                <Badge v-if="trackingTypeNames[detail.tracking_type]" variant="outline">
                  {{ trackingTypeNames[detail.tracking_type] }}
                </Badge>
              </div>
              <div class="text-sm">
                <p class="text-muted-foreground">{{ t('exercises.targetMuscle') }}:</p>
                <p class="font-medium">{{ detail.target_muscle.name }}</p>
              </div>
              <div v-if="detail.secondary_muscles?.length" class="text-sm">
                <p class="text-muted-foreground">{{ t('exercises.secondaryMuscles') }}:</p>
                <p class="font-medium">
                  {{ detail.secondary_muscles.map((m) => m.name).join(', ') }}
                </p>
              </div>
            </div>
          </div>

          <Separator />

          <div class="flex flex-col gap-2">
            <h3 class="text-sm font-semibold">{{ t('exercises.instructions') }}</h3>
            <ol
              v-if="detail.steps?.length"
              class="text-muted-foreground list-outside list-decimal space-y-1 pl-5 text-sm"
            >
              <li v-for="(step, i) in detail.steps" :key="i">{{ step }}</li>
            </ol>
            <p
              v-else-if="detail.instructions"
              class="text-muted-foreground text-sm whitespace-pre-line"
            >
              {{ detail.instructions }}
            </p>
            <p v-else class="text-muted-foreground text-sm">
              {{ t('exercises.noInstructions') }}
            </p>
          </div>
        </div>
      </template>
    </DialogScrollContent>
  </Dialog>
</template>
