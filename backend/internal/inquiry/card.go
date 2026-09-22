package inquiry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

const cardLength = 16

var persianArabicDigits = map[rune]rune{
	'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
	'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
	'٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
	'٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
}

func NormalizeCard(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if ascii, ok := persianArabicDigits[r]; ok {
			b.WriteRune(ascii)
			continue
		}
		switch r {
		case ' ', '-', '_', '.', '\t', '\u200c', '\u200f', '\u200e', '\u00a0':
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func ValidateCard(card string) error {
	switch {
	case card == "":
		return newValidationError("card", "شماره کارت را وارد کنید.")
	case !isAllDigits(card):
		return newValidationError("card", "شماره کارت باید فقط شامل ارقام باشد.")
	case len(card) != cardLength:
		return newValidationError("card", "شماره کارت باید دقیقاً ۱۶ رقم باشد.")
	case strings.HasPrefix(card, "0"):
		return newValidationError("card", "شماره کارت معتبر نیست.")
	case !luhnValid(card):
		return newValidationError("card", "شماره کارت معتبر نیست (خطای رقم کنترلی).")
	}
	return nil
}

func MaskCard(card string) string {
	if len(card) != cardLength {
		if len(card) <= 4 {
			return strings.Repeat("*", len(card))
		}
		return strings.Repeat("*", len(card)-4) + card[len(card)-4:]
	}
	return card[:6] + strings.Repeat("*", 6) + card[12:]
}

func FingerprintCard(card, secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(card))
	return hex.EncodeToString(mac.Sum(nil))
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func luhnValid(card string) bool {
	sum := 0
	double := false
	for i := len(card) - 1; i >= 0; i-- {
		d := int(card[i] - '0')
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
