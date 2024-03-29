package parsers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

func submitGoogleForm(items []chequeItem, t time.Time) (string, error) {
	var sum float32
	for _, item := range items {
		//dont overwhelm google
		time.Sleep(100 * time.Millisecond)

		resp, err := http.PostForm(os.Getenv("G_FORM_LINK"),
			url.Values{
				os.Getenv("G_FORM_TITLE_ENTRY"):     {item.Title},
				os.Getenv("G_FORM_PRICE_ENTRY"):     {fmt.Sprintf("%.2f", item.Price)},
				os.Getenv("G_FORM_CATEGORY_ENTRY"):  {os.Getenv("G_FORM_CATEGORY_D_VALUE")},
				os.Getenv("G_FORM_TIMESTAMP_ENTRY"): {t.Format("01/02/2006 15:04:05")},
			})

		if err != nil {
			return "", fmt.Errorf("error making google http request: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("POST status error: %v", resp.StatusCode)
		}

		sum += item.Price
	}

	return fmt.Sprintf("Submitted %d items, total cost is %.2f, timestamp is %s\n", len(items), sum, t.Format("02/01/2006 15:04:05")), nil
}
