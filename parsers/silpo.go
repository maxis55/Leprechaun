package silpo

import (
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type chequeItem struct {
	Title string
	Price float32
}

func ParseLink(url string) (result string, err error) {
	body, err := getHtml(url)
	if err != nil {
		return "", fmt.Errorf("HTTP error: %v", err)
	}

	items, t, err := parseHtml(body)

	if err != nil {
		return "", fmt.Errorf("parsing error: %v", err)
	}

	res, err := submitGoogleForm(items, t)

	if err != nil {
		return "", fmt.Errorf("google form error: %v", err)
	}

	return res, nil
}

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
				os.Getenv("G_FORM_TIMESTAMP_ENTRY"): {t.Format("02/01/2006 15:04:05")},
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

func getHtml(url string) (result string, error error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("GET error: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET status error: %v", resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}

	return string(bytes), nil
}

func parseHtml(htm string) ([]chequeItem, time.Time, error) {

	t, _ := html.Parse(strings.NewReader(htm))
	//t := html.NewTokenizer(strings.NewReader(htm))
	////var lineItems []chequeItem
	//
	//for {
	//	tokenType := t.Next()
	//
	//	switch tokenType {
	//	case html.StartTagToken, html.SelfClosingTagToken:
	//		token := t.Token()
	//
	//		// Check for all <li> element
	//		if token.Data == "table" && len(token.Attr) > 0 && strings.Contains(token.Attr[0].Val, "cheque-goods") {
	//			// Process the details of the Pokémon within this <li> element
	//			t.Next()
	//			//t.Next()
	//			//td := t.Raw()
	//			fmt.Println("token")
	//			fmt.Println(t.Raw())
	//			fmt.Println(t.Text())
	//			fmt.Println(string(t.Raw()))
	//			fmt.Println(string(t.Text()))
	//			//fmt.Println(td.Data)
	//			//fmt.Println(td.String())
	//			//fmt.Println(td.DataAtom)
	//			fmt.Println("tokenOver")
	//			//processPokemonDetails(z)
	//
	//			// Exit the loop after processing the details
	//			//return
	//		}
	//	default:
	//	}
	//
	//	if tokenType == html.ErrorToken {
	//		break
	//	}
	//}
	//
	//s := "<html><body><p>Some content</p></body></html>"
	//node, err := html.Parse(strings.NewReader(s))
	//if err != nil {
	//	panic(err.Error())
	//}
	//
	//// Root node
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	//// Step deeper
	//node = node.FirstChild
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	//// Step deeper
	//node = node.FirstChild
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	//// Step over to sibling
	//node = node.NextSibling
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	//// Step deeper
	//node = node.FirstChild
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	//// Step deeper
	//node = node.FirstChild
	//fmt.Printf("NodeType=%s Data=%s\n", nodeTypeAsString(node.Type), node.Data)
	var chequeItems []chequeItem
	var dateTime time.Time

	var processChequeGoods func(*html.Node) error
	processChequeGoods = func(n *html.Node) error {
		//by the time we've found dateTime
		//we should've already found chequeItem
		//don't care about anything else
		if !dateTime.IsZero() {
			return nil
		}
		//fmt.Printf("NodeType=%s Data=%s Attrs=%s\n", nodeTypeAsString(n.Type), n.Data, n.Attr)
		if n.Type == html.ElementNode && n.Data == "table" && len(n.Attr) > 0 && n.Attr[0].Val == "cheque-goods" {
			var err error
			chequeItems, err = getChequeItems(n)

			if err != nil {
				return fmt.Errorf("getting cheque items error: %v", err)
			}
		}

		if n.Type == html.ElementNode && n.Data == "div" && len(n.Attr) > 0 && strings.Contains(n.Attr[0].Val, "device-info-line") {
			var err error
			divWTitle := n.FirstChild.NextSibling
			//if column says that it contains time(and date)
			if strings.Contains(divWTitle.FirstChild.Data, "ЧАС") {
				//"9/30/2023 14:54:08" export format
				//"16:59:18 10.03.2024" import format
				dateTime, err = time.Parse("15:04:05 02.01.2006", divWTitle.NextSibling.NextSibling.FirstChild.Data)
				if err != nil {
					return fmt.Errorf("getting timestamp error: %v", err)
				}
			}
			return nil

		}

		// traverse the child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			err := processChequeGoods(c)
			if err != nil {
				return err
			}
		}
		return nil
	}
	err := processChequeGoods(t)
	if err != nil {
		return nil, dateTime, err
	}

	return chequeItems, dateTime, nil

}

func getChequeItems(table *html.Node) ([]chequeItem, error) {
	var lineItems []chequeItem

	tr := table.FirstChild.FirstChild

	for i := 0; ; i++ {
		if tr == nil {
			break
		}
		//fmt.Printf("NodeType=%s Data=%s Attrs=%s i=%d\n", nodeTypeAsString(tr.Type), tr.Data, tr.Attr, i)
		if i > 200 {
			return nil, fmt.Errorf("got stuck in infinite loop when getting cheque items")
		}
		//sometimes there are empty textNodes instead of trs because of Go's broken html parsing
		if tr.FirstChild == nil {
			tr = tr.NextSibling
			continue
		}

		td := tr.FirstChild.NextSibling

		//sometimes there are rows with just 1 td title, usually related to alcohol or specialized items
		//they have a product code, it is not useful
		if td == nil {
			tr = tr.NextSibling
			continue
		}
		//fmt.Printf("NodeType=%s Data=%s Attrs=%s i=%d\n", nodeTypeAsString(td.Type), td.Data, td.Attr, i)
		var currLineItem chequeItem

		if td.Type == html.ElementNode && len(td.Attr) > 0 && strings.Contains(td.Attr[0].Val, "no-break") {
			currLineItem.Title = td.FirstChild.Data

			//tr, text node, td --> textNode(title), text node, td --> textNode(price)
			if td.NextSibling != nil && td.NextSibling.NextSibling != nil {
				tdPrice := td.NextSibling.NextSibling.FirstChild
				price, err := strconv.ParseFloat(tdPrice.Data, 32)
				if err != nil {
					return nil, fmt.Errorf("parsing float in the first tr: %v %s", err, tdPrice.Data)
				}
				currLineItem.Price = float32(price)
			} else {
				//tr, text node, td, textNode(title)
				//tr, text node, td --> textNode(price explained), textNode, td --> textNode(price)
				td = tr.NextSibling.FirstChild.NextSibling
				currLineItem.Title += " " + td.FirstChild.Data
				tdPrice := td.NextSibling.NextSibling.FirstChild

				price, err := strconv.ParseFloat(tdPrice.Data, 32)
				if err != nil {
					return nil, fmt.Errorf("parsing float in the next tr: %v %s", err, tdPrice.Data)
				}
				currLineItem.Price = float32(price)
				tr = tr.NextSibling
			}
			lineItems = append(lineItems, currLineItem)
		}

		tr = tr.NextSibling
	}
	return lineItems, nil
}

func nodeTypeAsString(nodeType html.NodeType) string {
	switch nodeType {
	case html.ErrorNode:
		return "ErrorNode"
	case html.TextNode:
		return "TextNode"
	case html.DocumentNode:
		return "DocumentNode"
	case html.ElementNode:
		return "ElementNode"
	case html.CommentNode:
		return "CommentNode"
	case html.DoctypeNode:
		return "DoctypeNode"
	}
	return "UNKNOWN"
}
