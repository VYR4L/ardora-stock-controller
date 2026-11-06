package services

import "testing"

func TestBRLCurrencyRoundTrip(t *testing.T) {
	cases := map[string]int64{"R$ 1.234,56": 123456, "12,3": 1230, "0": 0, "19.90": 1990}
	for input, want := range cases {
		got, err := ParseBRLCents(input)
		if err != nil || got != want {
			t.Fatalf("ParseBRLCents(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
	if got := FormatBRLCents(123456); got != "R$ 1.234,56" {
		t.Fatalf("format = %q", got)
	}
	for _, invalid := range []string{"-1", "abc", "1,234"} {
		if _, err := ParseBRLCents(invalid); err == nil {
			t.Fatalf("expected error for %q", invalid)
		}
	}
}
