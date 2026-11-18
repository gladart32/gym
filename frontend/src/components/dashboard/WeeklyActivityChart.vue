<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BarController,
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  LinearScale,
  Tooltip,
} from 'chart.js'
import { Bar } from 'vue-chartjs'

import { formatTonnage } from '@/utils/format'

const props = defineProps({
  // [{ week_start, workouts, sets_done, total_volume_kg }]
  weeks: { type: Array, required: true },
})

const { t, locale } = useI18n()

ChartJS.register(BarController, BarElement, CategoryScale, LinearScale, Tooltip)

// Цвет баров берём из CSS-переменной темы (oklch), с запасным значением.
function themeColor(name, fallback) {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name)
  return value.trim() || fallback
}

const primary = themeColor('--primary', '#171717')

const shortWeek = (dateStr) => {
  const [y, m, d] = dateStr.split('-').map(Number)
  return new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(
    new Date(y, m - 1, d),
  )
}

const chartData = computed(() => ({
  labels: props.weeks.map((w) => shortWeek(w.week_start)),
  datasets: [
    {
      label: t('dashboard.activityMetric'),
      data: props.weeks.map((w) => w.workouts),
      backgroundColor: primary,
      borderRadius: 6,
      maxBarThickness: 28,
      categoryPercentage: 0.7,
    },
  ],
}))

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    y: {
      beginAtZero: true,
      ticks: { precision: 0, maxTicksLimit: 6 },
      grid: { drawTicks: false },
      border: { display: false },
    },
    x: { grid: { display: false }, border: { display: false } },
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        title: (items) => {
          const week = props.weeks[items[0].dataIndex]
          return `${shortWeek(week.week_start)} — ${t('dashboard.week')}`
        },
        label: (item) => {
          const week = props.weeks[item.dataIndex]
          return [
            t('dashboard.activityMetric') + `: ${week.workouts}`,
            t('dashboard.setsMetric') + `: ${week.sets_done}`,
            t('dashboard.volumeMetric') +
              `: ${formatTonnage(week.total_volume_kg, locale.value, t('dashboard.kg'), t('dashboard.tons'))}`,
          ]
        },
      },
    },
  },
}))
</script>

<template>
  <div class="h-56">
    <Bar :data="chartData" :options="chartOptions" />
  </div>
</template>
