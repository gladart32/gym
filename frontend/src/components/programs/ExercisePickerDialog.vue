<script setup>
import { ref, watch } from 'vue'
import { Plus, Search } from '@lucide/vue'
import { toast } from 'vue-sonner'

import { exercisesApi } from '@/api/exercises'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

const open = defineModel('open', { type: Boolean, default: false })

const emit = defineEmits(['select'])

const q = ref('')
const items = ref([])
const loading = ref(false)

let qTimer = null

async function search() {
  loading.value = true
  try {
    const params = { per_page: 20, sort: 'name' }
    const query = q.value.trim()
    if (query) params.q = query
    const data = await exercisesApi.list(params)
    items.value = data.items ?? []
  } catch (error) {
    toast.error(error.message)
  } finally {
    loading.value = false
  }
}

watch(open, (value) => {
  if (value && items.value.length === 0) search()
})

watch(q, () => {
  clearTimeout(qTimer)
  qTimer = setTimeout(search, 400)
})

function pick(exercise) {
  emit('select', exercise)
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ $t('programs.pickerTitle') }}</DialogTitle>
        <DialogDescription>{{ $t('programs.pickerHint') }}</DialogDescription>
      </DialogHeader>

      <div class="relative">
        <Search
          class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
        />
        <Input
          v-model="q"
          :placeholder="$t('exercises.searchPlaceholder')"
          class="pl-8"
        />
      </div>

      <div class="flex max-h-96 flex-col gap-1 overflow-y-auto">
        <div v-if="loading" class="flex flex-col gap-2 py-1">
          <Skeleton v-for="i in 5" :key="i" class="h-12 w-full" />
        </div>

        <p
          v-else-if="items.length === 0"
          class="text-muted-foreground py-8 text-center text-sm"
        >
          {{ $t('programs.pickerEmpty') }}
        </p>

        <template v-else>
          <button
            v-for="exercise in items"
            :key="exercise.id"
            type="button"
            class="hover:bg-muted flex items-center gap-3 rounded-lg p-2 text-left transition-colors"
            @click="pick(exercise)"
          >
            <div
              class="bg-muted flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-md"
            >
              <img
                v-if="exercise.media?.gif_url ?? exercise.media?.thumbnail_url"
                :src="exercise.media.gif_url ?? exercise.media.thumbnail_url"
                :alt="exercise.name"
                loading="lazy"
                decoding="async"
                class="size-full object-contain"
                @error="$event.target.style.visibility = 'hidden'"
              />
            </div>
            <div class="flex min-w-0 flex-1 flex-col gap-1">
              <p class="line-clamp-1 text-sm font-medium">{{ exercise.name }}</p>
              <div class="flex flex-wrap gap-1">
                <Badge variant="secondary" class="text-xs">{{ exercise.body_part.name }}</Badge>
                <Badge variant="outline" class="text-xs">{{ exercise.equipment.name }}</Badge>
              </div>
            </div>
            <Button variant="ghost" size="icon-sm" :aria-label="$t('programs.pickerAdd')">
              <Plus />
            </Button>
          </button>
        </template>
      </div>
    </DialogContent>
  </Dialog>
</template>
