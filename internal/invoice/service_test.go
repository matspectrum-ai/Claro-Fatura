package invoice

import (
	"reflect"
	"testing"
	"time"
)

func TestPhoneVariantsWithNinthDigit(t *testing.T) {
	got := PhoneVariants("93991234567")
	want := []string{"5593991234567", "9391234567", "93991234567"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PhoneVariants() = %#v, want %#v", got, want)
	}
}

func TestPhoneVariantsWithoutNinthDigit(t *testing.T) {
	got := PhoneVariants("9312345678")
	want := []string{"559312345678", "9312345678", "93912345678"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PhoneVariants() = %#v, want %#v", got, want)
	}
}

func TestMonthBoundsUTC(t *testing.T) {
	start, end := MonthBoundsUTC(time.Date(2026, time.September, 15, 20, 0, 0, 0, time.FixedZone("-03", -3*60*60)))
	if start != "2026-09-01" || end != "2026-09-30" {
		t.Fatalf("MonthBoundsUTC() = %s..%s", start, end)
	}
}

func TestDigits(t *testing.T) {
	if got := Digits("(93) 99123-4567"); got != "93991234567" {
		t.Fatalf("Digits() = %q", got)
	}
}
