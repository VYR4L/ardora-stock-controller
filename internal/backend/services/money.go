package services

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseBRLCents converts a Brazilian currency display value to integer cents.
func ParseBRLCents(value string) (int64, error) {
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "R$", ""), "\u00a0", ""))
	clean = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, clean)
	if clean == "" {
		return 0, nil
	}
	if strings.HasPrefix(clean, "-") {
		return 0, fmt.Errorf("o valor não pode ser negativo")
	}

	var whole, fraction string
	if strings.Contains(clean, ",") {
		if strings.Count(clean, ",") != 1 {
			return 0, fmt.Errorf("valor monetário inválido")
		}
		parts := strings.SplitN(clean, ",", 2)
		whole = strings.ReplaceAll(parts[0], ".", "")
		fraction = parts[1]
	} else if strings.Count(clean, ".") == 1 {
		parts := strings.SplitN(clean, ".", 2)
		if len(parts[1]) <= 2 {
			whole, fraction = parts[0], parts[1]
		} else {
			whole = strings.Join(parts, "")
		}
	} else {
		whole = strings.ReplaceAll(clean, ".", "")
	}
	if whole == "" {
		whole = "0"
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("use no máximo duas casas decimais")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	if fraction == "" {
		fraction = "00"
	}
	if !allDigits(whole) || !allDigits(fraction) {
		return 0, fmt.Errorf("valor monetário inválido")
	}
	reais, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("valor monetário inválido")
	}
	centavos, _ := strconv.ParseInt(fraction, 10, 64)
	if reais > (int64(^uint64(0)>>1)-centavos)/100 {
		return 0, fmt.Errorf("valor monetário muito alto")
	}
	return reais*100 + centavos, nil
}

// FormatBRLCents formats integer cents for display without floating point.
func FormatBRLCents(cents int64) string {
	if cents < 0 {
		return "-" + FormatBRLCents(-cents)
	}
	whole := strconv.FormatInt(cents/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "." + whole[i:]
	}
	return fmt.Sprintf("R$ %s,%02d", whole, cents%100)
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
