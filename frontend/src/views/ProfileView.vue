<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { Monitor, Smartphone, Trash2 } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'

import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input, PasswordInput } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useAuthStore } from '@/stores/auth'
import { useProfileStore } from '@/stores/profile'

const { t, te, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const profileStore = useProfileStore()
// серверные данные — из стора (кэш переживает уход со страницы)
const { currentMeasurement, measurements, measurementsLoading, equipmentItems, equipmentLoading } =
  storeToRefs(profileStore)

const sessions = ref([])
const sessionsLoading = ref(true)

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordLoading = ref(false)

// черновик формы профиля
const gender = ref(null)
const birthDate = ref('')
const targetWeight = ref('')
const fitnessLevel = ref(null)
const profileSaving = ref(false)

// черновик формы замера
const measurementDate = ref(todayString())
const measurementWeight = ref('')
const measurementHeight = ref('')
const measurementSaving = ref(false)

// черновой выбор инвентаря: правки живут локально до сохранения
const selectedEquipment = ref([])
const equipmentTouched = ref(false)
const equipmentSaving = ref(false)

const fitnessLevels = [
  { value: 'beginner', hintKey: 'profile.levelBeginnerHint' },
  { value: 'intermediate', hintKey: 'profile.levelIntermediateHint' },
  { value: 'advanced', hintKey: 'profile.levelAdvancedHint' },
]

const dateTimeFmt = computed(
  () => new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }),
)
const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }))

function todayString() {
  const d = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function formatLastUsed(session) {
  return session.last_used_at ? dateTimeFmt.value.format(new Date(session.last_used_at)) : ''
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

// первый приход списка замеров — предзаполнить форму выбранной даты;
// фоновые обновления стора форму не трогают
watch(measurements, (m, old) => {
  if (old === null && m !== null) syncMeasurementForm(measurementDate.value)
})

const currentBody = computed(() => {
  const m = currentMeasurement.value
  const weight = m?.weight_kg != null ? `${m.weight_kg} ${t('profile.kg')}` : t('profile.notSet')
  const height = m?.height_cm != null ? `${m.height_cm} ${t('profile.cm')}` : t('profile.notSet')
  return t('profile.currentBody', { weight, height })
})

// В карточке профиля показываем только 3 последних замера,
// полная история — на отдельной странице.
const measurementsList = computed(() => measurements.value ?? [])
const recentMeasurements = computed(() => measurementsList.value.slice(0, 3))
const hasMoreMeasurements = computed(() => measurementsList.value.length > 3)

function equipmentName(item) {
  return locale.value === 'ru' ? item.name_ru || item.name_en : item.name_en || item.name_ru
}

// Подсказка «что это такое» из локалей, по code из каталога;
// если для кода нет перевода — тултип просто не показываем.
const equipmentOptions = computed(() =>
  (equipmentItems.value ?? []).map((item) => {
    const key = `profile.equipmentHints.${item.code}`
    return { ...item, hint: te(key, locale.value) ? t(key) : '' }
  }),
)

// черновой выбор инвентаря синхронизируется из стора, пока пользователь
// не начал менять его сам
watch(
  () => profileStore.selectedEquipment,
  (ids) => {
    if (!equipmentTouched.value) selectedEquipment.value = [...ids]
  },
  { immediate: true },
)

async function loadSessions() {
  sessionsLoading.value = true
  try {
    sessions.value = await authApi.sessions()
  } catch (error) {
    toast.error(error.message)
  } finally {
    sessionsLoading.value = false
  }
}

function prefillProfileFields() {
  const p = profileStore.profile
  gender.value = p?.gender ?? null
  birthDate.value = p?.birth_date ? p.birth_date.slice(0, 10) : ''
  targetWeight.value = p?.target_weight_kg != null ? String(p.target_weight_kg) : ''
  fitnessLevel.value = p?.fitness_level ?? null
}

// форма предзаполняется один раз при появлении данных и после сохранения:
// фоновый refresh стора не затирает недосохранённое редактирование
const profileFormFilled = ref(false)
watch(
  () => profileStore.profile,
  (p) => {
    if (!p || profileFormFilled.value) return
    profileFormFilled.value = true
    prefillProfileFields()
  },
  { immediate: true },
)

async function saveProfile() {
  profileSaving.value = true
  try {
    const payload = {
      gender: gender.value,
      birth_date: birthDate.value || null,
      target_weight_kg: targetWeight.value === '' ? null : Number(targetWeight.value),
      fitness_level: fitnessLevel.value,
      tz_offset: new Date().getTimezoneOffset(),
    }
    const ok = await profileStore.saveProfile(payload)
    if (ok) {
      prefillProfileFields()
      toast.success(t('profile.profileSaved'))
    }
  } finally {
    profileSaving.value = false
  }
}

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

function toggleEquipment(id) {
  equipmentTouched.value = true
  selectedEquipment.value = selectedEquipment.value.includes(id)
    ? selectedEquipment.value.filter((x) => x !== id)
    : [...selectedEquipment.value, id]
}

async function saveEquipment() {
  equipmentSaving.value = true
  try {
    const ok = await profileStore.saveEquipment(selectedEquipment.value)
    if (ok) {
      // после сохранения черновик снова следует за стором
      equipmentTouched.value = false
      selectedEquipment.value = [...profileStore.selectedEquipment]
      toast.success(t('profile.equipmentSaved'))
    }
  } finally {
    equipmentSaving.value = false
  }
}

async function onRevoke(session) {
  try {
    await authApi.revokeSession(session.id)
    toast.success(t('profile.sessionTerminated'))
    await loadSessions()
  } catch (error) {
    toast.error(error.message)
  }
}

async function onRevokeAll() {
  try {
    await authApi.revokeAllSessions()
    toast.success(t('profile.allSessionsTerminated'))
    await loadSessions()
  } catch (error) {
    toast.error(error.message)
  }
}

async function onChangePassword() {
  if (newPassword.value !== confirmPassword.value) {
    toast.error(t('auth.toastPasswordsMismatch'))
    return
  }
  passwordLoading.value = true
  try {
    await authApi.changePassword(currentPassword.value, newPassword.value)
    toast.success(t('profile.passwordChanged'))
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (error) {
    toast.error(error.message)
  } finally {
    passwordLoading.value = false
  }
}

async function onLogout() {
  await auth.logout()
  toast.info(t('auth.loggedOut'))
  router.push('/auth/login')
}

onMounted(() => {
  loadSessions()
  profileStore.loadProfile()
  profileStore.loadMeasurements()
  profileStore.loadEquipment()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <h1 class="text-3xl font-bold">{{ $t('nav.profile') }}</h1>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.account') }}</CardTitle>
        <CardDescription>{{ auth.user?.email ?? '' }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div class="grid max-w-md gap-4 sm:grid-cols-2">
          <div class="grid content-start gap-2">
            <Label>{{ $t('profile.gender') }}</Label>
            <div class="flex gap-2">
              <Button
                :variant="gender === 'male' ? 'default' : 'outline'"
                size="sm"
                @click="gender = gender === 'male' ? null : 'male'"
              >
                {{ $t('profile.genderMale') }}
              </Button>
              <Button
                :variant="gender === 'female' ? 'default' : 'outline'"
                size="sm"
                @click="gender = gender === 'female' ? null : 'female'"
              >
                {{ $t('profile.genderFemale') }}
              </Button>
            </div>
          </div>
          <div class="grid gap-2">
            <Label for="birth-date">{{ $t('profile.birthDate') }}</Label>
            <Input id="birth-date" v-model="birthDate" type="date" :max="todayString()" />
          </div>
          <div class="grid gap-2">
            <Label for="target-weight">{{ $t('profile.targetWeight') }}</Label>
            <Input
              id="target-weight"
              v-model="targetWeight"
              type="number"
              step="0.1"
              min="20"
              max="500"
            />
          </div>
          <div class="grid content-start gap-2">
            <Label>{{ $t('profile.fitnessLevel') }}</Label>
            <div class="flex flex-wrap gap-2">
              <Button
                v-for="level in fitnessLevels"
                :key="level.value"
                :variant="fitnessLevel === level.value ? 'default' : 'outline'"
                size="sm"
                :title="$t(level.hintKey)"
                @click="fitnessLevel = fitnessLevel === level.value ? null : level.value"
              >
                {{ $t(`profile.level_${level.value}`) }}
              </Button>
            </div>
          </div>
        </div>
        <p class="text-muted-foreground text-sm">{{ currentBody }}</p>
        <Button class="w-fit" :disabled="profileSaving" @click="saveProfile">
          {{ $t('common.save') }}
        </Button>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.measurements') }}</CardTitle>
        <CardDescription>{{ $t('profile.measurementsDescription') }}</CardDescription>
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
            <tr v-for="m in recentMeasurements" :key="m.id" class="border-b">
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
        <Button v-if="hasMoreMeasurements" variant="outline" size="sm" class="w-fit" as-child>
          <RouterLink to="/profile/measurements">
            {{ $t('profile.viewAllMeasurements') }}
          </RouterLink>
        </Button>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.equipment') }}</CardTitle>
        <CardDescription>{{ $t('profile.equipmentDescription') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <p v-if="equipmentLoading" class="text-muted-foreground text-sm">
          {{ $t('common.loading') }}
        </p>
        <div v-else class="max-w-2xl">
          <TooltipProvider :delay-duration="200">
            <div class="flex flex-wrap gap-2">
              <Tooltip v-for="item in equipmentOptions" :key="item.id">
                <TooltipTrigger as-child>
                  <Button
                    :variant="selectedEquipment.includes(item.id) ? 'default' : 'outline'"
                    size="sm"
                    @click="toggleEquipment(item.id)"
                  >
                    {{ equipmentName(item) }}
                  </Button>
                </TooltipTrigger>
                <TooltipContent v-if="item.hint" class="max-w-60 text-wrap">
                  {{ item.hint }}
                </TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
        </div>
        <Button
          class="w-fit"
          :disabled="equipmentLoading || equipmentSaving"
          @click="saveEquipment"
        >
          {{ $t('common.save') }}
        </Button>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.security') }}</CardTitle>
        <CardDescription>{{ $t('profile.changePasswordDescription') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="flex max-w-sm flex-col gap-4" @submit.prevent="onChangePassword">
          <div class="grid gap-2">
            <Label for="current-password">{{ $t('profile.currentPassword') }}</Label>
            <PasswordInput
              id="current-password"
              v-model="currentPassword"
              autocomplete="current-password"
              required
            />
          </div>
          <div class="grid gap-2">
            <Label for="new-password">{{ $t('profile.newPassword') }}</Label>
            <PasswordInput
              id="new-password"
              v-model="newPassword"
              autocomplete="new-password"
              minlength="8"
              required
            />
          </div>
          <div class="grid gap-2">
            <Label for="confirm-password">{{ $t('auth.confirmPassword') }}</Label>
            <PasswordInput
              id="confirm-password"
              v-model="confirmPassword"
              autocomplete="new-password"
              minlength="8"
              required
            />
          </div>
          <Button type="submit" class="w-fit" :disabled="passwordLoading">
            {{ passwordLoading ? $t('common.loading') : t('profile.savePassword') }}
          </Button>
        </form>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>{{ $t('profile.sessions') }}</CardTitle>
        <CardDescription>{{ $t('profile.sessionsDescription') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <p v-if="sessionsLoading" class="text-muted-foreground text-sm">
          {{ $t('common.loading') }}
        </p>
        <p v-else-if="sessions.length === 0" class="text-muted-foreground text-sm">—</p>
        <ul v-else class="flex flex-col">
          <li
            v-for="session in sessions"
            :key="session.id"
            class="flex items-center justify-between gap-4 py-3"
          >
            <div class="flex min-w-0 items-center gap-3">
              <component
                :is="session.client_type === 'mobile' ? Smartphone : Monitor"
                class="text-muted-foreground size-5 shrink-0"
              />
              <div class="min-w-0">
                <p class="truncate text-sm font-medium">
                  {{ session.device_name || session.client_type }}
                  <span
                    v-if="session.is_current"
                    class="bg-primary/10 text-primary ml-2 rounded-full px-2 py-0.5 text-xs"
                  >
                    {{ $t('profile.thisDevice') }}
                  </span>
                </p>
                <p class="text-muted-foreground truncate text-xs">
                  {{ [session.os, session.app_version, session.ip].filter(Boolean).join(' · ') }}
                  · {{ $t('profile.lastUsed') }}: {{ formatLastUsed(session) }}
                </p>
              </div>
            </div>
            <Button v-if="session.is_current" variant="outline" size="sm" @click="onLogout">
              {{ $t('auth.logout') }}
            </Button>
            <Button v-else variant="outline" size="sm" @click="onRevoke(session)">
              {{ $t('profile.terminate') }}
            </Button>
          </li>
        </ul>
        <template v-if="sessions.length > 1">
          <Separator class="my-2" />
          <Button variant="outline" size="sm" @click="onRevokeAll">
            {{ $t('profile.terminateAll') }}
          </Button>
        </template>
      </CardContent>
    </Card>
  </div>
</template>
