<script setup>
import { computed, ref } from 'vue'
import { Heart } from '@lucide/vue'
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

const { t } = useI18n()

const props = defineProps({
  exercise: { type: Object, required: true },
})

defineEmits(['open', 'toggle-favorite'])

const mediaError = ref(false)

const attributionShort = computed(() => {
  const full = props.exercise.media?.attribution
  if (!full) return ''
  return full.split('—')[0].trim()
})

const favoriteLabel = computed(() =>
  props.exercise.is_favorite ? t('exercises.removeFromFavorites') : t('exercises.addToFavorites'),
)
</script>

<template>
  <Card
    class="group hover:border-ring cursor-pointer overflow-hidden py-0 transition-colors"
    @click="$emit('open', exercise)"
  >
    <div class="bg-muted relative flex aspect-square items-center justify-center overflow-hidden">
      <img
        v-if="exercise.media?.gif_url && !mediaError"
        :src="exercise.media.gif_url"
        :alt="exercise.name"
        loading="lazy"
        decoding="async"
        class="size-full object-contain"
        @error="mediaError = true"
      />
      <span v-else class="text-muted-foreground px-4 text-center text-xs">
        {{ t('exercises.noMedia') }}
      </span>
      <span
        v-if="exercise.media?.gif_url && !mediaError && attributionShort"
        class="bg-background/70 text-muted-foreground absolute right-1 bottom-1 rounded px-1 text-[10px] leading-4"
      >
        {{ attributionShort }}
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        class="bg-background/70 absolute top-1 right-1"
        :aria-label="favoriteLabel"
        :title="favoriteLabel"
        @click.stop="$emit('toggle-favorite', exercise)"
      >
        <Heart
          class="size-4 transition-colors"
          :class="exercise.is_favorite ? 'fill-destructive text-destructive' : 'text-muted-foreground'"
        />
      </Button>
    </div>
    <CardContent class="flex flex-col gap-2 p-3">
      <p class="line-clamp-2 min-h-10 text-sm leading-5 font-medium">
        {{ exercise.name }}
      </p>
      <div class="flex flex-wrap gap-1">
        <Badge variant="secondary">{{ exercise.body_part.name }}</Badge>
        <Badge variant="outline">{{ exercise.equipment.name }}</Badge>
      </div>
    </CardContent>
  </Card>
</template>
