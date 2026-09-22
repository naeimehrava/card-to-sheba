<script setup lang="ts">
import { computed } from 'vue'
import { formatCard, toAsciiDigits } from '~/utils/card'

const props = defineProps<{
  modelValue: string
  disabled?: boolean
  errorMessage?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  submit: []
}>()

const displayValue = computed(() => formatCard(props.modelValue))
const hasError = computed(() => Boolean(props.errorMessage))

function onInput(event: Event) {
  const input = event.target as HTMLInputElement
  const digits = toAsciiDigits(input.value).slice(0, 16)

  emit('update:modelValue', digits)

  input.value = formatCard(digits)
}
</script>

<template>
  <div>
    <label for="card-number" class="theme-muted block text-sm font-medium">
      شماره کارت
    </label>

    <input
      id="card-number"
      :value="displayValue"
      type="text"
      inputmode="numeric"
      autocomplete="cc-number"
      dir="ltr"
      placeholder="6037 9975 1234 5670"
      maxlength="19"
      :disabled="disabled"
      :aria-invalid="hasError"
      :aria-describedby="hasError ? 'card-number-error' : 'card-number-hint'"
      class="theme-input numeric mt-2 w-full rounded-xl border px-3 py-3 text-base tracking-wider
             disabled:cursor-not-allowed disabled:opacity-60
             transition focus:border-sky-500 sm:px-4 sm:text-lg sm:tracking-widest"
      :class="hasError ? 'theme-input-error' : ''"
      @input="onInput"
      @keyup.enter="emit('submit')"
    >

    <p v-if="hasError" id="card-number-error" class="mt-2 text-sm leading-6 text-rose-600" role="alert">
      {{ errorMessage }}
    </p>
    <p v-else id="card-number-hint" class="theme-muted mt-2 text-sm">
      ۱۶ رقم روی کارت را وارد کنید. ارقام فارسی و فاصله هم پذیرفته می‌شود.
    </p>
  </div>
</template>
