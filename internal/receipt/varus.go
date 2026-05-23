package receipt

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Varus receipt structure (relevant parts):
//
//   <div id="bot"><div id="table"><table>
//     <tr class="tabletitle">...</tr>        (header, skipped — selector targets tr.service only)
//     <tr class="service">
//       <td class="item">
//         <p class="itemtext"><span>Штрих код 4820...</span></p>   (barcode, skipped)
//         <p class="itemtext">Title text</p>
//         <p class="itemtext">Знижка</p>                            (only on discounted items, skipped)
//       </td>
//       <td class="tableitem" ...>            (quantity, e.g. "1.000" / "0.274")
//         <p class="itemtext">1.000</p>
//       </td>
//       <td class="tableitem" ...>            (price)
//         <p class="itemtext">79.90   А</p>   (gross, kept; trailing tax letter stripped)
//         <p class="itemtext">0.59   А</p>    (discount amount, present only on discounted items, skipped)
//       </td>
//     </tr>
//   </table></div>
//
// Timestamp:
//   <p class="fscl-info-bot">01.05.2026 19:33</p>

// "01.05.2026 19:33"
var varusDateTimeRe = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}$`)

// "79.90   А" -> "79.90"
var varusPriceRe = regexp.MustCompile(`\d+\.\d+`)

// ParseVarus extracts items and the receipt timestamp from a Varus cheque HTML
// document. Conforms to the Parser type so it can be passed to Process.
func ParseVarus(htm string) ([]Item, time.Time, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htm))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("html parse: %v", err)
	}

	var items []Item
	var rowErr error
	doc.Find("tr.service").EachWithBreak(func(_ int, tr *goquery.Selection) bool {
		item, err := parseVarusItemRow(tr)
		if err != nil {
			rowErr = fmt.Errorf("item row: %v", err)
			return false
		}
		items = append(items, item)
		return true
	})
	if rowErr != nil {
		return nil, time.Time{}, rowErr
	}

	t, err := parseVarusTimestamp(doc)
	if err != nil {
		return nil, time.Time{}, err
	}

	if len(items) == 0 {
		return nil, t, fmt.Errorf("no items found")
	}
	return items, t, nil
}

func parseVarusItemRow(tr *goquery.Selection) (Item, error) {
	var item Item

	tds := tr.ChildrenFiltered("td")
	if tds.Length() < 3 {
		return item, fmt.Errorf("expected >=3 <td>, got %d", tds.Length())
	}

	// Title: first <p class="itemtext"> in the item cell that has no child elements
	// (barcode paragraphs wrap their text in a <span>) and isn't "Знижка".
	titleCell := tds.Eq(0)
	title := ""
	titleCell.ChildrenFiltered("p.itemtext").EachWithBreak(func(_ int, p *goquery.Selection) bool {
		if p.Children().Length() > 0 {
			return true
		}
		t := strings.TrimSpace(p.Text())
		if t == "" || t == "Знижка" {
			return true
		}
		title = t
		return false
	})
	if title == "" {
		return item, fmt.Errorf("no title in item cell")
	}

	// Quantity (К-сть): first non-empty <p class="itemtext"> in the middle cell.
	// Mirrors the Silpo convention of appending the quantity string to the title in-line.
	qty := strings.TrimSpace(tds.Eq(1).ChildrenFiltered("p.itemtext").First().Text())
	if qty != "" && qty != "1.000" {
		title = title + " " + qty
	}
	item.Title = title

	// Price: first <p class="itemtext"> in the last cell; second (if any) is the discount, ignored.
	priceText := strings.TrimSpace(tds.Eq(tds.Length() - 1).ChildrenFiltered("p.itemtext").First().Text())
	match := varusPriceRe.FindString(priceText)
	if match == "" {
		return item, fmt.Errorf("no numeric price in %q", priceText)
	}
	price, err := parsePrice(match)
	if err != nil {
		return item, err
	}
	item.Price = price
	return item, nil
}

func parseVarusTimestamp(doc *goquery.Document) (time.Time, error) {
	var dateTime time.Time
	doc.Find("p.fscl-info-bot").EachWithBreak(func(_ int, p *goquery.Selection) bool {
		text := strings.TrimSpace(p.Text())
		if !varusDateTimeRe.MatchString(text) {
			return true
		}
		t, err := time.Parse("02.01.2006 15:04", text)
		if err != nil {
			return true
		}
		dateTime = t
		return false
	})
	if dateTime.IsZero() {
		return dateTime, fmt.Errorf("no receipt timestamp found")
	}
	return dateTime, nil
}
