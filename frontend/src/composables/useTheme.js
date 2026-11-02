import { useDark, useToggle } from '@vueuse/core'

const isDark = useDark()
const toggleTheme = useToggle(isDark)

export function useTheme() {
  return { isDark, toggleTheme }
}
