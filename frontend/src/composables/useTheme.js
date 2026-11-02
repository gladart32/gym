import { useColorMode } from '@vueuse/core'

const theme = useColorMode({
  storageKey: 'gym-theme',
  emitAuto: true,
  modes: { auto: '', light: '', dark: 'dark' },
})

export function useTheme() {
  return { theme }
}
