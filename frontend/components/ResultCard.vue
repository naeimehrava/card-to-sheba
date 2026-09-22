<script setup lang="ts">import { ref } from 'vue'
import { formatMaskedCard, ibanChunks } from '~/utils/card'
import type { InquiryResult } from '~/composables/useCardToSheba'

const props = defineProps<{ result: InquiryResult }>()

const copied = ref(false)

async function copyIban() {
  try {
    await navigator.clipboard.writeText(props.result.iban.replace(/\s+/g, ''))
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    copied.value = false
  }
}

const details = computed(() => {
  const rows: { label: string; value: string; numeric?: boolean }[] = []
  if (props.result.bankName) rows.push({ label: 'بانک', value: props.result.bankName })
  if (props.result.depositOwners) rows.push({ label: 'صاحب حساب', value: props.result.depositOwners })
  if (props.result.deposit) rows.push({ label: 'شماره حساب', value: props.result.deposit, numeric: true })
  if (props.result.maskedCard) {
    rows.push({ label: 'شماره کارت', value: formatMaskedCard(props.result.maskedCard), numeric: true })
  }
  return rows
})
</script>

<template>
  <section class="theme-success overflow-hidden rounded-2xl border" role="status" aria-live="polite">
    <div class="flex items-start justify-between gap-3 px-4 py-3 sm:px-5">
      <h2 class="theme-success-title text-sm font-semibold leading-snug">
        استعلام با موفقیت انجام شد
      </h2>
      <span class="theme-success-label shrink-0 text-xs" aria-hidden="true">✓</span>
    </div>

    <div class="theme-success-line border-t px-4 py-4 sm:px-5">
      <p class="theme-success-label text-xs">شماره شبا</p>

      <div class="mt-2 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p
          class="numeric min-w-0 select-all font-mono text-[15px] font-semibold leading-7 tracking-wide sm:text-lg sm:leading-8"
          dir="ltr"
        >
          <span
            v-for="(chunk, i) in ibanChunks(result.iban)"
            :key="i"
            class="theme-success-text me-1 inline-block last:me-0"
          >{{ chunk }}</span>
        </p>

        <button
          type="button"
          class="theme-copy min-h-10 w-full shrink-0 rounded-xl border px-4 text-sm font-medium transition sm:min-h-9 sm:w-auto sm:rounded-lg sm:px-3 sm:text-xs"
          @click="copyIban"
        >
          {{ copied ? 'کپی شد' : 'کپی شبا' }}
        </button>
      </div>
    </div>

    <dl
      v-if="details.length"
      class="theme-success-line theme-success-grid grid grid-cols-1 gap-px border-t sm:grid-cols-2"
    >
      <div
        v-for="row in details"
        :key="row.label"
        class="theme-success min-w-0 px-4 py-3 sm:px-5"
        :class="!row.numeric && row.value.length > 22 ? 'sm:col-span-2' : ''"
      >
        <dt class="theme-success-label text-xs">{{ row.label }}</dt>
        <dd
          class="theme-success-text mt-1 break-words text-sm leading-6"
          :class="row.numeric ? 'numeric font-mono' : ''"
          :dir="row.numeric ? 'ltr' : undefined"
        >
          {{ row.value }}
        </dd>
      </div>
    </dl>
  </section>
</template>
