package parsers

import (
	"fmt"
	"golang.org/x/net/html"
	"regexp"
	"strings"
	"time"
)

// Varus receipt structure (relevant parts):
//
//   <div id="bot"><div id="table"><table>
//     <tr class="tabletitle">...</tr>        (skipped — header)
//     <tr class="service">                   (one per item)
//       <td class="item">
//         <p class="itemtext"><span>Штрих код 4820...</span></p>
//         <p class="itemtext">Title text</p>
//       </td>
//       <td class="tableitem" ...>           (quantity)
//         <p class="itemtext">1.000</p>
//       </td>
//       <td class="tableitem" ...>           (line total + tax letter, e.g. "79.90   А")
//         <p class="itemtext">79.90   А</p>
//       </td>
//     </tr>
//     ...
//   </table></div>
//
// Timestamp lives later in the document:
//   <p class="fscl-info-bot">01.05.2026 19:33</p>

// "01.05.2026 19:33"
var varusDateTimeRe = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}$`)

// "79.90   А" -> "79.90"
var varusPriceRe = regexp.MustCompile(`[\d]+\.[\d]+`)

func ParseVarusChequeHtml(htm string) ([]ChequeItem, time.Time, error) {
	root, err := html.Parse(strings.NewReader(htm))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("html parse: %v", err)
	}

	var items []ChequeItem
	var dateTime time.Time

	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "tr" && hasClass(n, "service") {
			item, err := parseVarusItemRow(n)
			if err != nil {
				return fmt.Errorf("item row: %v", err)
			}
			items = append(items, item)
			return nil
		}

		if dateTime.IsZero() && n.Type == html.ElementNode && n.Data == "p" && hasClass(n, "fscl-info-bot") {
			text := strings.TrimSpace(textContent(n))
			if varusDateTimeRe.MatchString(text) {
				t, err := time.Parse("02.01.2006 15:04", text)
				if err != nil {
					return fmt.Errorf("timestamp parse: %v", err)
				}
				dateTime = t
				return nil
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(root); err != nil {
		return nil, dateTime, err
	}

	if dateTime.IsZero() {
		return nil, dateTime, fmt.Errorf("no receipt timestamp found")
	}
	if len(items) == 0 {
		return nil, dateTime, fmt.Errorf("no items found")
	}

	return items, dateTime, nil
}

func parseVarusItemRow(tr *html.Node) (ChequeItem, error) {
	var item ChequeItem

	tds := childrenByTag(tr, "td")
	if len(tds) < 3 {
		return item, fmt.Errorf("expected >=3 <td>, got %d", len(tds))
	}

	// Title cell contains, in order: a barcode <p> (with <span> child), the title <p>,
	// and optionally a "Знижка" (discount) <p>. The price cell mirrors this: the first
	// numeric line is the gross price, the second is the discount amount.
	// We record the gross price and ignore the discount.
	title, err := varusItemTitle(tds[0])
	if err != nil {
		return item, err
	}

	// Quantity (К-сть): single decimal in the middle cell, e.g. "1.000" or "0.274".
	// Append non-unit quantities to the title in-line — matches the Silpo convention
	// where "2 X 55.49" gets concatenated onto the title field.
	qty := firstNonEmptyItemText(tds[1])
	if qty != "" && qty != "1.000" {
		title = title + " " + qty
	}
	item.Title = title

	priceTexts := itemTextParagraphs(tds[len(tds)-1])
	if len(priceTexts) == 0 {
		return item, fmt.Errorf("no price paragraphs in item row")
	}
	match := varusPriceRe.FindString(priceTexts[0])
	if match == "" {
		return item, fmt.Errorf("no numeric price in %q", priceTexts[0])
	}
	price, err := parsePrice(match)
	if err != nil {
		return item, err
	}
	item.Price = price

	return item, nil
}

func firstNonEmptyItemText(td *html.Node) string {
	for _, t := range itemTextParagraphs(td) {
		if s := strings.TrimSpace(t); s != "" {
			return s
		}
	}
	return ""
}

// varusItemTitle picks the human title out of the first <td> of an item row.
// It skips the barcode paragraph (the one containing a <span>) and the trailing
// "Знижка" paragraph if present.
func varusItemTitle(td *html.Node) (string, error) {
	var candidates []string
	for c := td.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Data != "p" || !hasClass(c, "itemtext") {
			continue
		}
		// Skip the barcode paragraph (its content is wrapped in a <span>).
		if firstElementChild(c) != nil {
			continue
		}
		text := strings.TrimSpace(textContent(c))
		if text == "" {
			continue
		}
		if text == "Знижка" {
			continue
		}
		candidates = append(candidates, text)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no title paragraph in item cell")
	}
	// If the cell ever carries multiple non-barcode, non-discount paragraphs,
	// the first one is the product title; later ones are extra notes we don't model.
	return candidates[0], nil
}

func firstElementChild(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
			return false
		}
	}
	return false
}

func childrenByTag(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			out = append(out, c)
		}
	}
	return out
}

// itemTextParagraphs collects the text of every <p class="itemtext"> directly under n.
// Bare <span> children inside those <p>s are flattened into the text.
func itemTextParagraphs(n *html.Node) []string {
	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "p" && hasClass(c, "itemtext") {
			out = append(out, textContent(c))
		}
	}
	return out
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}
