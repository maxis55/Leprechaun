package receipt

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func sumPrices(items []Item) float32 {
	var s float32
	for _, it := range items {
		s += it.Price
	}
	return s
}

// closeEnough is needed because we sum float32 prices line by line: the result
// can drift by ~1¢ from the integer-arithmetic total printed on the receipt.
func closeEnough(a, b float32) bool {
	return math.Abs(float64(a-b)) < 0.05
}

func TestParseSilpo_Basic(t *testing.T) {
	items, ts, err := ParseSilpo(loadFixture(t, "silpo_basic.html"))
	if err != nil {
		t.Fatalf("ParseSilpo: %v", err)
	}

	if got, want := len(items), 5; got != want {
		t.Errorf("item count: got %d, want %d", got, want)
	}
	if got, want := sumPrices(items), float32(194.72); !closeEnough(got, want) {
		t.Errorf("total: got %.2f, want %.2f", got, want)
	}

	wantTS := time.Date(2026, 5, 20, 19, 49, 0, 0, time.UTC)
	if !ts.Equal(wantTS) {
		t.Errorf("timestamp: got %s, want %s", ts, wantTS)
	}

	// First item is a 2-row (inline title + price) shape — basic case.
	if items[0].Title != "Нап0.33SchwIndTonЖ/б" || !closeEnough(items[0].Price, 30.99) {
		t.Errorf("first item: got %+v, want title 'Нап0.33SchwIndTonЖ/б' @ 30.99", items[0])
	}

	// Last item is the 3-row weighed shape (БананКг + "0.768 X 73.90" qty row).
	// The qty string must be appended to the title in the same field.
	last := items[len(items)-1]
	if last.Title != "БананКг 0.768 X 73.90" || !closeEnough(last.Price, 56.76) {
		t.Errorf("last item: got %+v, want title 'БананКг 0.768 X 73.90' @ 56.76", last)
	}
}

func TestParseVarus_Basic(t *testing.T) {
	items, ts, err := ParseVarus(loadFixture(t, "varus_basic.html"))
	if err != nil {
		t.Fatalf("ParseVarus: %v", err)
	}

	if got, want := len(items), 19; got != want {
		t.Errorf("item count: got %d, want %d", got, want)
	}
	if got, want := sumPrices(items), float32(1612.72); !closeEnough(got, want) {
		t.Errorf("total: got %.2f, want %.2f", got, want)
	}

	wantTS := time.Date(2026, 5, 1, 19, 33, 0, 0, time.UTC)
	if !ts.Equal(wantTS) {
		t.Errorf("timestamp: got %s, want %s", ts, wantTS)
	}

	// First item is unit-quantity (1.000) — qty must NOT be appended.
	if items[0].Title != "ІкоркаЛососемВоднийСвіт250г" {
		t.Errorf("first item title: got %q, want unit-qty form without suffix", items[0].Title)
	}

	// Weighed item ("Цибуля Марс, ваг 0.474") — qty suffix is appended.
	const weighedTitle = "Цибуля Марс, ваг 0.474"
	var found bool
	for _, it := range items {
		if it.Title == weighedTitle {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing weighed item with appended qty %q", weighedTitle)
	}
}

func TestParseVarus_Discounted(t *testing.T) {
	// This receipt has a "Знижка" paragraph and a second price line per row.
	// Both must be ignored so the parser yields gross prices and clean titles.
	items, _, err := ParseVarus(loadFixture(t, "varus_discounted.html"))
	if err != nil {
		t.Fatalf("ParseVarus: %v", err)
	}

	if got, want := len(items), 18; got != want {
		t.Errorf("item count: got %d, want %d", got, want)
	}
	if got, want := sumPrices(items), float32(614.57); !closeEnough(got, want) {
		t.Errorf("gross total: got %.2f, want %.2f", got, want)
	}

	// Sanity: no item title should equal the discount marker, since that's the
	// exact regression we're guarding against.
	for i, it := range items {
		if it.Title == "Знижка" {
			t.Errorf("item %d picked up 'Знижка' as title, indicates title-paragraph filter regression", i)
		}
	}
}
