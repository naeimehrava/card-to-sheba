package inquiry

import (
	"encoding/json"
	"strings"
)

var sensitiveJSONKeys = map[string]bool{
	"card":       true,
	"cardnumber": true,
	"pan":        true,
	"cardno":     true,
}

func sanitizeUpstreamJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	cleaned, err := json.Marshal(maskSensitive(doc))
	if err != nil {
		return ""
	}
	return string(cleaned)
}

func maskSensitive(node any) any {
	switch v := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			if s, ok := value.(string); ok && sensitiveJSONKeys[strings.ToLower(key)] {
				out[key] = MaskCard(s)
				continue
			}
			out[key] = maskSensitive(value)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = maskSensitive(item)
		}
		return out
	default:
		return node
	}
}

func MaskIBAN(iban string) string {
	iban = strings.TrimSpace(iban)
	if len(iban) <= 8 {
		return strings.Repeat("*", len(iban))
	}
	return iban[:4] + strings.Repeat("*", len(iban)-8) + iban[len(iban)-4:]
}
