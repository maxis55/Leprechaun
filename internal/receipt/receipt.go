// Package receipt models a parsed grocery receipt and orchestrates the full
// fetch -> parse -> submit pipeline for each supported retailer.
package receipt

import (
	"fmt"
	"strconv"
	"time"

	"leprechaun/internal/form"
	"leprechaun/internal/httpx"
)

// Item is one row from a receipt that will become one row in the Google Form.
// Quantity (for weighed and multi-unit purchases) is appended to Title inline
// — there is no separate amount field on the form. Discounts are not modeled:
// the parsers record the gross price and skip the discount lines entirely.
type Item struct {
	Title string
	Price float32
}

// Parser turns receipt HTML into a list of items plus the receipt timestamp.
type Parser func(html string) ([]Item, time.Time, error)

// Process runs the full pipeline for a given retailer URL and returns the
// human-readable summary that the bot replies to Discord with.
func Process(url string, parse Parser) (string, error) {
	body, err := httpx.GetHTML(url)
	if err != nil {
		return "", fmt.Errorf("fetch: %v", err)
	}

	items, t, err := parse(body)
	if err != nil {
		return "", fmt.Errorf("parse: %v", err)
	}

	res, err := form.Submit(toForm(items), t)
	if err != nil {
		return "", fmt.Errorf("submit: %v", err)
	}
	return res, nil
}

// toForm bridges the receipt domain type to the form package's input shape,
// keeping form/ ignorant of where its data comes from.
func toForm(items []Item) []form.Entry {
	out := make([]form.Entry, len(items))
	for i, it := range items {
		out[i] = form.Entry{Title: it.Title, Price: it.Price}
	}
	return out
}

// parsePrice is shared between the Silpo and Varus parsers. Lives here (and not
// in httpx/) because it understands receipt prices specifically — decimal-dot
// floats fitting in float32.
func parsePrice(s string) (float32, error) {
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0, fmt.Errorf("parse price %q: %v", s, err)
	}
	return float32(v), nil
}
