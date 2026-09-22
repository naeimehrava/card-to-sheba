export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'theme'

function writeDom(next: Theme) {
  const root = document.documentElement
  root.classList.toggle('dark', next === 'dark')
  root.dataset.theme = next
  root.style.colorScheme = next
}

export function useTheme() {
  const theme = useState<Theme>('theme', () => 'light')

  function apply(next: Theme) {
    theme.value = next
    if (!import.meta.client) return
    writeDom(next)
    localStorage.setItem(STORAGE_KEY, next)
  }

  function toggle() {
    apply(theme.value === 'dark' ? 'light' : 'dark')
  }

  onMounted(() => {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'dark' || saved === 'light') {
      apply(saved)
      return
    }
    apply('light')
  })

  return { theme, toggle }
}
