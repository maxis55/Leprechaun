// redact reduces a saved receipt HTML to the minimal subset that the parsers
// exercise, so the fixture is safe to commit. Everything else — cashier names,
// card PAN, RRN/auth/terminal IDs, loyalty numbers, QR/HMAC payloads, inline
// SVG/CSS, the JS bundle — is dropped. Product barcodes are replaced with a
// placeholder.
//
// Usage (from repo root):
//
//	# 1. Download the full receipt into the testdata dir, e.g.
//	curl -sf -A "$UA" "https://ecom-gateway.varus.ua/public/api/e-receipt/view/<uuid>" \
//	  -o internal/receipt/testdata/varus_<something>.html
//
//	# 2. Strip PII in place:
//	go run ./internal/receipt/testdata/redact
//
// The tool walks every *.html file directly under internal/receipt/testdata and
// rewrites it based on whether the filename starts with "silpo_" or "varus_".
// New files following that naming convention are picked up automatically.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const testdataDir = "internal/receipt/testdata"

func main() {
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		die("read testdata: %v", err)
	}

	var processed int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		path := filepath.Join(testdataDir, e.Name())
		switch {
		case strings.HasPrefix(e.Name(), "silpo_"):
			if err := redactSilpo(path); err != nil {
				die("%s: %v", path, err)
			}
		case strings.HasPrefix(e.Name(), "varus_"):
			if err := redactVarus(path); err != nil {
				die("%s: %v", path, err)
			}
		default:
			fmt.Printf("skip %s (unknown retailer prefix)\n", path)
			continue
		}
		processed++
		fmt.Printf("redacted %s\n", path)
	}
	fmt.Printf("done (%d files)\n", processed)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func redactSilpo(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	if err != nil {
		return err
	}

	goods := doc.Find("table.cheque-goods").First()
	if goods.Length() == 0 {
		return fmt.Errorf("no cheque-goods table")
	}
	goodsHTML, _ := goquery.OuterHtml(goods)

	// Pull only the ЧАС label cell + value cell. Everything else in the
	// device-info block (terminal IDs, fiscal numbers, HMAC, online/offline)
	// is discarded.
	var tsRow string
	doc.Find(".device-info-line-item").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if !strings.Contains(s.Text(), "ЧАС") {
			return true
		}
		val := strings.TrimSpace(s.Next().Text())
		tsRow = fmt.Sprintf(
			`<table><tr><td class="device-info-line-item">ЧАС :</td><td class="device-info-line-item">%s</td></tr></table>`,
			val,
		)
		return false
	})
	if tsRow == "" {
		return fmt.Errorf("no ЧАС row")
	}

	out := fmt.Sprintf(`<!doctype html><html><body>
<!-- Redacted Silpo receipt fixture: only parser-relevant nodes retained. -->
%s
<div class="cheque-device-info">%s</div>
</body></html>`, goodsHTML, tsRow)

	return os.WriteFile(path, []byte(out), 0644)
}

var varusTimestampRe = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}$`)

func redactVarus(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	if err != nil {
		return err
	}

	// Keep every <tr class="service">. Replace barcode <span>s with a placeholder
	// — they leak the product EAN, which is enough to look up exact products and
	// (combined with the timestamp + store) the purchase itself.
	var serviceRows []string
	doc.Find("tr.service").Each(func(_ int, s *goquery.Selection) {
		s.Find("p.itemtext").Each(func(_ int, p *goquery.Selection) {
			if span := p.Find("span"); span.Length() > 0 {
				span.SetText("Штрих код XXXXXXXXXXXXX")
			}
		})
		h, _ := goquery.OuterHtml(s)
		serviceRows = append(serviceRows, h)
	})
	if len(serviceRows) == 0 {
		return fmt.Errorf("no <tr class=\"service\"> rows")
	}

	// Timestamp paragraph (matches strict dd.MM.yyyy HH:mm shape only).
	var ts string
	doc.Find("p.fscl-info-bot").EachWithBreak(func(_ int, p *goquery.Selection) bool {
		t := strings.TrimSpace(p.Text())
		if varusTimestampRe.MatchString(t) {
			ts = t
			return false
		}
		return true
	})
	if ts == "" {
		return fmt.Errorf("no fscl-info-bot timestamp")
	}

	out := fmt.Sprintf(`<!doctype html><html><body>
<!-- Redacted Varus receipt fixture: only parser-relevant nodes retained.
     Barcode spans replaced with placeholder. -->
<div id="bot"><div id="table"><table>
%s
</table></div></div>
<p class="fscl-info-bot">%s</p>
</body></html>`, strings.Join(serviceRows, "\n"), ts)

	return os.WriteFile(path, []byte(out), 0644)
}
