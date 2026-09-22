const PERSIAN_ARABIC_DIGITS: Record<string, string> = {
  '۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
  '۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
  '٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
  '٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
}

export function toAsciiDigits(input: string): string {
  return Array.from(input)
    .map((ch) => PERSIAN_ARABIC_DIGITS[ch] ?? ch)
    .join('')
    .replace(/\D/g, '')
}

export function formatCard(digits: string): string {
  return digits.replace(/(\d{4})(?=\d)/g, '$1 ').trim()
}

export function formatMaskedCard(masked: string): string {
  return masked.replace(/(.{4})(?=.)/g, '$1 ')
}

export function isPlausibleCard(digits: string): boolean {
  if (!/^\d{16}$/.test(digits) || digits.startsWith('0')) return false

  let sum = 0
  let double = false
  for (let i = digits.length - 1; i >= 0; i--) {
    let d = Number(digits[i])
    if (double) {
      d *= 2
      if (d > 9) d -= 9
    }
    sum += d
    double = !double
  }
  return sum % 10 === 0
}

export function ibanChunks(iban: string): string[] {
  const compact = iban.replace(/\s+/g, '')
  return compact.match(/.{1,4}/g) ?? [compact]
}
