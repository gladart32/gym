<script setup>
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'

defineProps({
  program: { type: Object, required: true },
})

defineEmits(['open'])
</script>

<template>
  <Card
    class="group hover:bg-muted/40 hover:border-ring hover:shadow-xs cursor-pointer py-0 transition-[background-color,border-color,box-shadow]"
    @click="$emit('open', program)"
  >
    <CardContent class="flex flex-col gap-2 p-4">
      <div class="flex items-start justify-between gap-2">
        <p class="line-clamp-2 min-h-10 text-sm leading-5 font-medium">{{ program.name }}</p>
        <Badge v-if="program.is_ready && !program.is_published" variant="secondary">
          {{ $t('programs.draft') }}
        </Badge>
      </div>
      <p v-if="program.description" class="text-muted-foreground line-clamp-2 text-sm">
        {{ program.description }}
      </p>
      <div class="flex flex-wrap items-center gap-1">
        <Badge v-if="program.level" variant="outline">
          {{ $t(`profile.level_${program.level}`) }}
        </Badge>
        <Badge v-if="program.copied_from_id" variant="outline">
          {{ $t('programs.copyBadge') }}
        </Badge>
      </div>
      <p class="text-muted-foreground text-xs">
        {{ $t('programs.daysCount', program.days_count) }} ·
        {{ $t('programs.exercisesCount', program.exercises_count) }}
      </p>
      <p v-if="program.author_name" class="text-muted-foreground text-xs">
        {{ program.author_name }}
      </p>
    </CardContent>
  </Card>
</template>
