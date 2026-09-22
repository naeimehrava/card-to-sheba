<script setup lang="ts">
const { theme, toggle } = useTheme()

const label = computed(() =>
  theme.value === 'dark' ? 'تغییر به حالت روشن' : 'تغییر به حالت تاریک',
)
</script>

<template>
  <button
    type="button"
    class="theme-toggle inline-flex min-h-10 items-center gap-1.5 rounded-xl border px-2.5 py-2 text-xs font-medium shadow-sm transition sm:min-h-9 sm:gap-2 sm:px-3"
    :aria-label="label"
    :title="label"
    @click="toggle"
  >
    <span aria-hidden="true" class="text-base leading-none">
      {{ theme === 'dark' ? '☀' : '☾' }}
    </span>
    {{ theme === 'dark' ? 'روشن' : 'تاریک' }}
  </button>
</template>
