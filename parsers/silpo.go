package parsers

import (
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ChequeItem struct {
	Title    string
	Price    float32
	Category string
}

// Silpo cheque structure (current as of May 2026):
//
//   <table class="cheque-goods">
//     <tbody>                         (one per item)
//       <tr><td class="cheque-row-lcolumn">ШК 5449000046390</td></tr>     (barcode row, skipped)
//
//       <!-- 2-row shape: title + price in same row -->
//       <tr>
//         <td class="cheque-row-lcolumn no-break">Нап0.33SchwIndTonЖ/б</td>
//         <td class="cheque-row-rcolumn">30.99</td>
//         <td class="cheque-row-rcolumn">A</td>
//       </tr>
//
//       <!-- 3-row shape (weighed/multi-unit items): title row has no price -->
//       <tr><td class="cheque-row-lcolumn no-break" style="padding-right: 0;">БананКг</td></tr>
//       <tr>
//         <td class="cheque-row-lcolumn no-break">0.768 X 73.90</td>     (quantity/unit line, appended to title)
//         <td class="cheque-row-rcolumn">56.76</td>                       (line total)
//         <td class="cheque-row-rcolumn">A</td>
//       </tr>
//     </tbody>
//   </table>
//
// Items whose title contains "уцінка" (markdown/clearance) are skipped — they
// represent a discount applied to a previous line, not a separate purchase.
//
// Timestamp lives in the device-info block:
//   <div class="cheque-device-info">
//     <table>
//       <tr><td class="device-info-line-item">ЧАС :</td>
//           <td class="device-info-line-item">19:49:00 20.05.2026</td></tr>
//     </table>
//   </div>

func ParseSilpoChequeHtml(htm string) ([]ChequeItem, time.Time, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htm))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("html parse: %v", err)
	}

	items, err := parseSilpoItems(doc)
	if err != nil {
		return nil, time.Time{}, err
	}

	dateTime, err := parseSilpoTimestamp(doc)
	if err != nil {
		return nil, time.Time{}, err
	}

	return items, dateTime, nil
}

func parseSilpoItems(doc *goquery.Document) ([]ChequeItem, error) {
	var items []ChequeItem
	var firstErr error

	doc.Find("table.cheque-goods > tbody").EachWithBreak(func(_ int, tb *goquery.Selection) bool {
		// Find the title row: the first <tr> whose first <td> has class "no-break".
		// Alcohol and other excise items prepend extra <tr>s with a UKT-ZED code and
		// an internal product code before the title row — scanning for the first
		// "no-break" row skips them. Do not replace this with `tb.Children().Eq(N)`
		// or the alcohol case will silently break.
		titleTr := tb.Find("tr").FilterFunction(func(_ int, tr *goquery.Selection) bool {
			return tr.Children().First().HasClass("no-break")
		}).First()
		if titleTr.Length() == 0 {
			return true // no goods row in this tbody (rare structural row)
		}

		titleCell := titleTr.Children().First()
		titleText := strings.TrimSpace(titleCell.Text())
		if titleText == "" || strings.Contains(titleText, "уцінка") {
			return true
		}

		var item ChequeItem
		item.Title = titleText

		// 2-row shape: title cell has sibling <td>s in the same row with the price.
		// 3-row shape: title row has only one <td>; the next <tr> carries "qty X unitprice"
		// and the price siblings live there.
		priceRow := titleTr
		if titleTr.Children().Length() == 1 {
			next := titleTr.Next()
			if next.Length() == 0 {
				return true
			}
			qtyCell := next.Children().First()
			item.Title = item.Title + " " + strings.TrimSpace(qtyCell.Text())
			priceRow = next
		}

		priceCell := priceRow.Children().Eq(1)
		if priceCell.Length() == 0 {
			firstErr = fmt.Errorf("missing price cell for %q", item.Title)
			return false
		}
		price, err := parsePrice(strings.TrimSpace(priceCell.Text()))
		if err != nil {
			firstErr = fmt.Errorf("parsing price for %q: %v", item.Title, err)
			return false
		}
		item.Price = price
		items = append(items, item)
		return true
	})

	if firstErr != nil {
		return nil, firstErr
	}
	return items, nil
}

func parseSilpoTimestamp(doc *goquery.Document) (time.Time, error) {
	var dateTime time.Time
	doc.Find(".device-info-line-item").EachWithBreak(func(_ int, label *goquery.Selection) bool {
		if !strings.Contains(label.Text(), "ЧАС") {
			return true
		}
		value := strings.TrimSpace(label.Next().Text())
		// Layout: "HH:mm:ss dd.MM.yyyy" e.g. "19:49:00 20.05.2026"
		t, err := time.Parse("15:04:05 02.01.2006", value)
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
