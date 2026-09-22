<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { isPlausibleCard } from '~/utils/card'

const card = ref('')
const { loading, result, error, submit, reset } = useCardToSheba()

const canSubmit = computed(() => isPlausibleCard(card.value) && !loading.value)

const inlineError = computed(() => {
  if (error.value?.code === 'VALIDATION_ERROR') return error.value.message
  return ''
})

watch(card, () => {
  if (result.value || error.value) reset()
})

function onSubmit() {
  if (!canSubmit.value) return
  submit(card.value)
}
</script>

<template>
  <main class="mx-auto flex min-h-dvh w-full max-w-xl flex-col justify-start px-3 py-5 sm:justify-center sm:px-6 sm:py-10">
    <header class="mb-5 sm:mb-8">
      <div class="mb-3 flex justify-end">
        <ThemeToggle />
      </div>
      <div class="text-center">
        <h1 class="theme-title text-xl font-bold leading-snug sm:text-3xl">
          تبدیل شماره کارت به شبا
        </h1>
        <p class="theme-muted mt-1.5 text-xs leading-6 sm:mt-2 sm:text-sm">
          شماره کارت بانکی را وارد کنید تا شماره شبا نمایش داده شود.
        </p>
      </div>
    </header>

    <div class="theme-card w-full min-w-0 overflow-hidden rounded-2xl border p-4 shadow-sm sm:p-8">
      <form novalidate @submit.prevent="onSubmit">
        <CardInput
          v-model="card"
          :disabled="loading"
          :error-message="inlineError"
          @submit="onSubmit"
        />

        <button
          type="submit"
          :disabled="!canSubmit"
          class="theme-submit mt-5 flex min-h-12 w-full items-center justify-center gap-2 rounded-xl px-4
                 text-sm font-semibold transition disabled:cursor-not-allowed sm:mt-6 sm:min-h-11"
        >
          <span
            v-if="loading"
            class="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"
            aria-hidden="true"
          />
          {{ loading ? 'در حال استعلام…' : 'استعلام شبا' }}
        </button>
      </form>

      <p class="sr-only" aria-live="polite">
        {{ loading ? 'در حال استعلام' : '' }}
      </p>

      <div v-if="result || error" class="mt-5 min-w-0 sm:mt-6">
        <ResultCard v-if="result" :result="result" />
        <ErrorAlert v-else-if="error" :error="error" />
      </div>
    </div>
  </main>
</template>
