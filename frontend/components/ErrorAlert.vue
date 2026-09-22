<script setup lang="ts">import { computed } from 'vue'
import type { InquiryError } from '~/composables/useCardToSheba'

const props = defineProps<{ error: InquiryError }>()

const isUserFixable = computed(() => props.error.code === 'VALIDATION_ERROR')

const isRetryable = computed(() =>
  ['UPSTREAM_TIMEOUT', 'UPSTREAM_UNAVAILABLE', 'NETWORK_ERROR'].includes(props.error.code),
)
</script>

<template>
  <section
    class="rounded-2xl border p-4 sm:p-5"
    :class="isUserFixable ? 'theme-warn' : 'theme-error'"
    role="alert"
    aria-live="assertive"
  >
    <h2
      class="text-sm font-semibold"
      :class="isUserFixable ? 'theme-warn-title' : 'theme-error-title'"
    >
      {{ isUserFixable ? 'ورودی نامعتبر' : 'استعلام انجام نشد' }}
    </h2>

    <p class="mt-2 text-sm" :class="isUserFixable ? 'theme-warn-text' : 'theme-error-text'">
      {{ error.message }}
    </p>

    <p v-if="isRetryable" class="mt-2 text-xs" :class="isUserFixable ? 'theme-warn-text' : 'theme-error-text'">
      می‌توانید چند لحظه بعد دوباره تلاش کنید.
    </p>

    <p v-if="error.requestId" class="theme-muted numeric mt-3 break-all font-mono text-[11px] leading-5">
      کد پیگیری: {{ error.requestId }}
    </p>
  </section>
</template>
