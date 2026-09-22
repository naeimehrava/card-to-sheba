import { ref } from 'vue'
import { toAsciiDigits } from '~/utils/card'

export interface InquiryResult {
  maskedCard: string
  iban: string
  bankName?: string
  deposit?: string
  depositOwners?: string
}

export interface InquiryError {
  code: string
  message: string
  fields?: { field: string; message: string }[]
  requestId?: string
}

interface ApiErrorBody {
  error?: { code?: string; message?: string; fields?: { field: string; message: string }[] }
  requestId?: string
}

const NETWORK_ERROR: InquiryError = {
  code: 'NETWORK_ERROR',
  message: 'ارتباط با سرور برقرار نشد. اتصال اینترنت خود را بررسی کنید.',
}

export function useCardToSheba() {
  const config = useRuntimeConfig()

  const loading = ref(false)
  const result = ref<InquiryResult | null>(null)
  const error = ref<InquiryError | null>(null)

  function reset() {
    result.value = null
    error.value = null
  }

  async function submit(rawCard: string) {
    reset()
    loading.value = true

    try {
      const response = await $fetch<{ data: InquiryResult; requestId: string }>(
        '/api/v1/card-to-iban',
        {
          baseURL: config.public.apiBaseUrl,
          method: 'POST',
          body: { card: toAsciiDigits(rawCard) },
        },
      )
      result.value = response.data
    } catch (e: unknown) {
      error.value = toInquiryError(e)
    } finally {
      loading.value = false
    }
  }

  return { loading, result, error, submit, reset }
}

function toInquiryError(e: unknown): InquiryError {
  const body = (e as { data?: ApiErrorBody })?.data
  if (!body?.error?.message) return NETWORK_ERROR

  return {
    code: body.error.code ?? 'UNKNOWN',
    message: body.error.message,
    fields: body.error.fields,
    requestId: body.requestId,
  }
}
