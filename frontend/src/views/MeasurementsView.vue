<script setup>
import { ArrowLeft, Trash2 } from '@lucide/vue'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useProfileStore } from '@/stores/profile'

const { t, locale } = useI18n()
const profileStore = useProfileStore()
// список замеров живёт в сторе и переживает уход со страницы
const { measurements, measurementsLoading } = storeToRefs(profileStore)

const measurementDate = ref(todayString())
const measurementWeight = ref('')
const measurementHeight = ref('')
const measurementSaving = ref(false)

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }))
const measurementsList = computed(() => measurements.value ?? [])

function todayString() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function formatDate(iso) {
  return dateFmt.value.format(new Date(iso))
}

// Форма — редактор выбранной даты: если за дату уже есть замер,
// поля предзаполняются его значениями (upsert перезаписывает запись целиком).
function syncMeasurementForm(date) {
  const existing = measurementsList.value.find((m) => m.measured_on.slice(0, 10) === date)
  measurementWeight.value = existing?.weight_kg != null ? String(existing.weight_kg) : ''
  measurementHeight.value = existing?.height_cm != null ? String(existing.height_cm) : ''
}

watch(measurementDate, syncMeasurementForm)

// первый приход данных — предзаполнить форму выбранной даты;
// фоновые обновления стора форму не трогают
watch(measurements, (m, old) => {
  if (old === null && m !== null) syncMeasurementForm(measurementDate.value)
})

async function addMeasurement() {
  if (measurementWeight.value === '' && measurementHeight.value === '') {
    toast.error(t('profile.measurementEmpty'))
    return
  }
  measurementSaving.value = true
  try {
    const ok = await profileStore.addMeasurement({
      measured_on: measurementDate.value,
      weight_kg: measurementWeight.value === '' ? null : Number(measurementWeight.value),
      height_cm: measurementHeight.value === '' ? null : Number(measurementHeight.value),
      tz_offset: new Date().getTimezoneOffset(),
    })
    if (ok) {
      toast.success(t('profile.measurementSaved'))
      syncMeasurementForm(measurementDate.value)
    }
  } finally {
    measurementSaving.value = false
  }
}

async function deleteMeasurement(measurement) {
  const ok = await profileStore.deleteMeasurement(measurement.id)
  if (ok) {
    toast.success(t('profile.measurementDeleted'))
    syncMeasurementForm(measurementDate.value)
  }
}

onMounted(() => profileStore.loadMeasurements())
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex items-center gap-4">
      <h1 class="text-3xl font-bold">{{ $t('profile.measurements') }}</h1>
      <Button variant="outline" size="sm" as-child>
        <RouterLink to="/profile">
          <ArrowLeft class="size-4" />
          {{ $t('profile.backToProfile') }}
        </RouterLink>
      </Button>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.measurements') }}</CardTitle>
        <CardDescription>{{ $t('profile.allMeasurementsDescription') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="addMeasurement">
          <div class="grid gap-1.5">
            <Label for="measurement-date">{{ $t('profile.measurementDate') }}</Label>
            <Input
              id="measurement-date"
              v-model="measurementDate"
              type="date"
              :max="todayString()"
              required
            />
          </div>
          <div class="grid gap-1.5">
            <Label for="measurement-weight">{{ $t('profile.measurementWeight') }}</Label>
            <Input
              id="measurement-weight"
              v-model="measurementWeight"
              type="number"
              step="0.1"
              min="20"
              max="500"
            />
          </div>
          <div class="grid gap-1.5">
            <Label for="measurement-height">{{ $t('profile.measurementHeight') }}</Label>
            <Input
              id="measurement-height"
              v-model="measurementHeight"
              type="number"
              step="0.1"
              min="50"
              max="300"
            />
          </div>
          <Button type="submit" :disabled="measurementSaving">
            {{ $t('common.save') }}
          </Button>
        </form>
        <p v-if="measurementsLoading" class="text-muted-foreground text-sm">
          {{ $t('common.loading') }}
        </p>
        <p v-else-if="measurementsList.length === 0" class="text-muted-foreground text-sm">
          {{ $t('profile.noMeasurements') }}
        </p>
        <table v-else class="w-full max-w-md text-sm">
          <thead>
            <tr class="text-muted-foreground border-b text-left">
              <th class="py-2 pr-4 font-normal">{{ $t('profile.measurementDate') }}</th>
              <th class="py-2 pr-4 font-normal">{{ $t('profile.measurementWeight') }}</th>
              <th class="py-2 pr-4 font-normal">{{ $t('profile.measurementHeight') }}</th>
              <th class="w-10"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in measurementsList" :key="m.id" class="border-b">
              <td class="py-2 pr-4">{{ formatDate(m.measured_on) }}</td>
              <td class="py-2 pr-4">
                {{ m.weight_kg != null ? `${m.weight_kg} ${$t('profile.kg')}` : '—' }}
              </td>
              <td class="py-2 pr-4">
                {{ m.height_cm != null ? `${m.height_cm} ${$t('profile.cm')}` : '—' }}
              </td>
              <td class="py-2 text-right">
                <Button
                  variant="destructive"
                  size="icon-sm"
                  :aria-label="$t('profile.measurementDeleted')"
                  @click="deleteMeasurement(m)"
                >
                  <Trash2 />
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </CardContent>
    </Card>
  </div>
</template>
