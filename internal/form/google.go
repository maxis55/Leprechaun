// Package form submits parsed receipt entries to a Google Form, one POST per
// entry. Configuration is environment-only; see main.go's requiredEnv list for
// the full set of keys.
package form

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Entry is the receipt-package-agnostic shape the form submitter accepts.
// Keeps form/ from depending on the receipt package and vice-versa.
type Entry struct {
	Title string
	Price float32
}

// Pause between submissions: Google has rate-limited bulk form submissions in
// the past, so a small per-item delay is intentional. Don't shrink this without
// testing under load.
const submitPause = 100 * time.Millisecond

// Submit posts each entry to the Google Form and returns the user-facing
// summary (item count, total, receipt timestamp). On any non-2xx response the
// receipt is aborted mid-flight; entries already submitted are not rolled back.
func Submit(entries []Entry, t time.Time) (string, error) {
	endpoint := os.Getenv("G_FORM_LINK")
	titleField := os.Getenv("G_FORM_TITLE_ENTRY")
	priceField := os.Getenv("G_FORM_PRICE_ENTRY")
	categoryField := os.Getenv("G_FORM_CATEGORY_ENTRY")
	categoryValue := os.Getenv("G_FORM_CATEGORY_D_VALUE")
	tsField := os.Getenv("G_FORM_TIMESTAMP_ENTRY")

	var sum float32
	for _, e := range entries {
		time.Sleep(submitPause)

		resp, err := http.PostForm(endpoint, url.Values{
			titleField:         {e.Title},
			priceField:         {fmt.Sprintf("%.2f", e.Price)},
			categoryField:      {categoryValue},
			tsField + "_year":  {t.Format("2006")},
			tsField + "_month": {t.Format("01")},
			tsField + "_day":   {t.Format("02")},
			tsField + "_hour":  {t.Format("15")},
			tsField + "_minute": {t.Format("04")},
			// As one field, if the form is reconfigured to a single datetime input:
			// tsField: {t.Format("01/02/2006 15:04:05")},
		})
		if err != nil {
			return "", fmt.Errorf("post %q: %v", e.Title, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("post %q: status %d", e.Title, resp.StatusCode)
		}

		sum += e.Price
	}

	return fmt.Sprintf("Submitted %d items, total cost is %.2f, timestamp is %s\n",
		len(entries), sum, t.Format("02/01/2006 15:04:05")), nil
}
