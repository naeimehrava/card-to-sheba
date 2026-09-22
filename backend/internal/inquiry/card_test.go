package inquiry

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeCard(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already normalized", "6037997512345670", "6037997512345670"},
		{"spaces and dashes", " 6037-9975 1234 5670 ", "6037997512345670"},
		{"persian digits", "۶۰۳۷۹۹۷۵۱۲۳۴۵۶۷۰", "6037997512345670"},
		{"arabic digits", "٦٠٣٧٩٩٧٥١٢٣٤٥٦٧٠", "6037997512345670"},
		{"mixed digits with separators", "۶۰۳۷-٩٩٧٥-1234-5670", "6037997512345670"},
		{"zero width non joiner", "6037\u200c9975\u200c1234\u200c5670", "6037997512345670"},
		{"non digits are kept for validation to reject", "6037abcd12345670", "6037abcd12345670"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeCard(tt.in); got != tt.want {
				t.Errorf("NormalizeCard(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateCard(t *testing.T) {
	tests := []struct {
		name    string
		card    string
		wantErr bool
	}{
		{"valid saman card", "6037997512345670", false},
		{"valid mellat card", "6104337012345672", false},
		{"empty", "", true},
		{"too short", "603799751234567", true},
		{"too long", "60379975123456701", true},
		{"non numeric", "6037abcd12345670", true},
		{"leading zero", "0037997512345670", true},
		{"fails luhn", "6037997512345678", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCard(tt.card)
			if tt.wantErr != (err != nil) {
				t.Fatalf("ValidateCard(%q) error = %v, wantErr = %v", tt.card, err, tt.wantErr)
			}
			if err == nil {
				return
			}

			var svcErr *Error
			if !errors.As(err, &svcErr) {
				t.Fatalf("ValidateCard returned %T, want *Error", err)
			}
			if svcErr.Kind != KindValidation {
				t.Errorf("Kind = %q, want %q", svcErr.Kind, KindValidation)
			}
			if len(svcErr.Fields) != 1 || svcErr.Fields[0].Field != "card" {
				t.Errorf("Fields = %+v, want a single error on the card field", svcErr.Fields)
			}
		})
	}
}

func TestMaskCard(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"6037997512345670", "603799******5670"},
		{"12345", "*2345"},
		{"123", "***"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := MaskCard(tt.in); got != tt.want {
			t.Errorf("MaskCard(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMaskCardNeverLeaksTheMiddleDigits(t *testing.T) {
	const card = "6037997512345670"
	masked := MaskCard(card)

	if masked == card {
		t.Fatal("masked card equals the original")
	}
	if got, want := len(masked), len(card); got != want {
		t.Errorf("masked length = %d, want %d", got, want)
	}
	if masked[6:12] != "******" {
		t.Errorf("middle segment = %q, want six mask characters", masked[6:12])
	}
}

func TestFingerprintCard(t *testing.T) {
	const card = "6037997512345670"

	if got := FingerprintCard(card, ""); got != "" {
		t.Errorf("FingerprintCard with no secret = %q, want empty", got)
	}

	first := FingerprintCard(card, "secret-a")
	if first == "" {
		t.Fatal("FingerprintCard returned empty for a configured secret")
	}
	if first != FingerprintCard(card, "secret-a") {
		t.Error("fingerprint is not stable for the same card and secret")
	}
	if first == FingerprintCard(card, "secret-b") {
		t.Error("fingerprint did not change with the secret")
	}
	if first == FingerprintCard("6104337012345672", "secret-a") {
		t.Error("fingerprint did not change with the card")
	}
	if contains(first, card) {
		t.Error("fingerprint contains the card number")
	}
}

func TestMaskIBAN(t *testing.T) {
	got := MaskIBAN("IR820540102680020817909002")
	if got[:4] != "IR82" {
		t.Errorf("prefix = %q, want IR82", got[:4])
	}
	if got[len(got)-4:] != "9002" {
		t.Errorf("suffix = %q, want 9002", got[len(got)-4:])
	}
	if got == "IR820540102680020817909002" {
		t.Error("IBAN was not masked")
	}
}

func TestSanitizeUpstreamJSONMasksCardFields(t *testing.T) {
	raw := []byte(`{"data":{"card":"6037997512345670","iban":"IR82","owners":[{"cardNumber":"6104337012345672"}]},"meta":{"code":0}}`)

	got := sanitizeUpstreamJSON(raw)

	if contains(got, "6037997512345670") || contains(got, "6104337012345672") {
		t.Fatalf("sanitized payload still contains a full card number: %s", got)
	}
	if !contains(got, "603799******5670") {
		t.Errorf("sanitized payload lost the masked card: %s", got)
	}
	if !contains(got, "IR82") {
		t.Errorf("sanitized payload dropped non-sensitive fields: %s", got)
	}
}

func TestSanitizeUpstreamJSONDropsNonJSON(t *testing.T) {
	if got := sanitizeUpstreamJSON([]byte("<html>gateway error 6037997512345670</html>")); got != "" {
		t.Errorf("sanitizeUpstreamJSON kept a non-JSON body: %q", got)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
