package parsers

import (
	"fmt"
	"golang.org/x/net/html"
	"strconv"
	"strings"
	"time"
)

type ChequeItem struct {
	Title    string
	Price    float32
	Category string
}

func ParseSilpoChequeHtml(htm string) ([]ChequeItem, time.Time, error) {

	t, _ := html.Parse(strings.NewReader(htm))
	//t := html.NewTokenizer(strings.NewReader(htm))
	////var lineItems []ChequeItem
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
	var chequeItems []ChequeItem
	var dateTime time.Time

	var processChequeGoods func(*html.Node) error
	processChequeGoods = func(n *html.Node) error {
		//by the time we've found dateTime
		//we should've already found ChequeItem
		//don't care about anything else
		if !dateTime.IsZero() {
			return nil
		}
		//fmt.Printf("NodeType=%s Data=%s Attrs=%s\n", nodeTypeAsString(n.Type), n.Data, n.Attr)
		if n.Type == html.ElementNode && n.Data == "table" && len(n.Attr) > 0 && n.Attr[0].Val == "cheque-goods" {
			var err error
			chequeItems, err = getSilpoChequeItems(n)

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

func getSilpoChequeItems(table *html.Node) ([]ChequeItem, error) {
	var lineItems []ChequeItem

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
		var currLineItem ChequeItem

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

//reference cheque

/*
<!DOCTYPE html><html><head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><style>body {    margin: 8px 0px;}table {    border-spacing: 0px;}td:last-child {    padding-left: 3px;}tr, td:first-child {    padding-left: 0px;}.cheque-page {    display: block}.cheque-b
lock {    width: 260px;    margin: 0px auto;    padding: 10px 20px;    font-family: 'Courier New', Courier, monospace;    border: 1px solid silver;    font-size: 13px;    line-height: 14px;}.cheque-header {    padding-bottom: 20px}.cheque-devider {    text-align: center;    white-space: nowrap;    overflow: hid
den;    margin: 5px 0}.no-break {    white-space: nowrap;}.centered {    text-align: center}.cheque-row {    display: flex;    flex-direction: row}.cheque-row-lcolumn {    width: 100%;    overflow: hidden}.cheque-row-rcolumn {    width: 280px;    text-align: right;    white-space: nowrap;    vertical-align: bot
tom;}td.cheque-row-lcolumn {    text-align: left;}td.cheque-row-rcolumn {    text-align: right;    width: auto;}.device-info-line {    display: flex;    flex-direction: row}.device-info-line-item {    margin-left: 5px}.device-info-line-item:first-child {    width: 80px;    text-align: right}.qrcode {    width:
122px;}.hmac {    margin-top: 24px;}</style></head><body><div class='cheque-block'>  <div class='cheque-header'>    <div class='centered'>ТОВ "Сільпо-Фуд"</div>    <div class='centered'>Кафе</div>    <div class='centered'>м. Київ</div>    <div class='cen
tered'>ПН 123123123</div></div>  <div>Чек N 111/22/222</div>  <div>Каса N2</div>  <table class='cheque-goods'><tr>      <td class='cheque-row-lcolumn'>УКТ ЗЕД 2208701000</td></tr>      <tr><td class='cheque-row-lcolumn'>AFOP938656</td></tr><tr>      <td class='cheque-row-lcolumn no-break'>Л
ікер Baileys Colada , 0<br />,7л</td>      <td class='cheque-row-rcolumn'>639.00</td><td class='cheque-row-rcolumn'>MA</td></tr><tr>      <td class='cheque-row-lcolumn'>УКТ ЗЕД 2208701000</td></tr>      <tr><td class='cheque-row-lcolumn'>AFHF514402</td></tr><tr>      <td class='cheque-row-lcolumn no-break'>Лiке
р Baileys Salted Car<br />amel , 0,7л</td>      <td class='cheque-row-rcolumn'>639.00</td><td class='cheque-row-rcolumn'>MA</td></tr><tr>      <td class='cheque-row-lcolumn no-break'>Нап500КакаоGelЩенПат</td></tr><tr>        <td class='cheque-row-lcolumn'>2 X 129.00</td>        <td class='cheque-row-rcolumn'>25
8.00</td><td class='cheque-row-rcolumn'>A</td></tr><tr>      <td class='cheque-row-lcolumn no-break'>Цукор1ПЧБілКрист3кат</td></tr><tr>        <td class='cheque-row-lcolumn'>2 X 31.99</td>        <td class='cheque-row-rcolumn'>63.98</td><td class='cheque-row-rcolumn'>A</td></tr><tr>      <td class='cheque-row-l
column no-break'>Мол900ПремLokПас2,5</td>      <td class='cheque-row-rcolumn'>33.99</td><td class='cheque-row-rcolumn'>A</td></tr><tr>      <td class='cheque-row-lcolumn no-break'>Мол2000ПремLokПас2,5</td>      <td class='cheque-row-rcolumn'>76.99</td><td class='cheque-row-rcolumn'>A</td></tr><tr>      <td clas
s='cheque-row-lcolumn no-break'>Хл330ЦарХлТостРанЦіл</td>      <td class='cheque-row-rcolumn'>26.19</td><td class='cheque-row-rcolumn'>A</td></tr></table> <div class='cheque-row'> <div class='cheque-row-lcolumn'>ПІДСУМОК</div> <div class='cheque-row-rcolumn'>1737.15</div> </div> <div class='cheque-row'> <div cl
ass='cheque-row-lcolumn'><b>ЗНИЖКА</b></div> <div class='cheque-row-rcolumn'><b>53.15</b></div> </div>  <div class='cheque-body'><div class='cheque-devider'>------------------------------------</div><div>АКЦІЇ</div><div>Ви сяєте</div><div>яскравіше за всіх.</div><div>ВЛАСНИЙ РАХУНОК</div><div>[123123123]</div><d
iv>балобонуси в моб. додатку</div>    <div class='cheque-devider'>------------------------------------</div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>СУМА</div>      <div class='cheque-row-rcolumn'>1684.00 ГРН</div></div>    <table class='cheque-taxes'><tr>      <td class='cheque-row-lco
lumn'>ПДВ A 20.00%</td>      <td class='cheque-row-rcolumn'>270.85</td><td>ГРН</td></tr><tr>      <td class='cheque-row-lcolumn'>Збір M/+A 6.00%</td>      <td class='cheque-row-rcolumn'>59.00</td><td>ГРН</td></tr></table></div>  <div class='cheque-devider'>------------------------------------</div>  <div class=
'payment-info'>    <div>      <div class='cheque-row'>        <div class='cheque-row-lcolumn'>КАРТКА</div>        <div class='cheque-row-rcolumn'>1684.00 ГРН</div>      </div>  <div class='cheque-devider'>------------------------------------</div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'
>ІДЕНТ.ЕКВАЙРА</div>      <div class='cheque-row-rcolumn'>QR2022</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>ТЕРМІНАЛ</div>      <div class='cheque-row-rcolumn'>QR2022</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>КОМІСІЯ</div>      <div class='che
que-row-rcolumn'>0.00</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>ПЛАТІЖНА СИСТЕМА</div>      <div class='cheque-row-rcolumn'>QR</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>ВИД ОПЕРАЦІЇ</div>      <div class='cheque-row-rcolumn'>ОПЛАТА</div></div
>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>ЕПЗ</div>      <div class='cheque-row-rcolumn'><span class='spanepz'>2222222</span></div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>КОД АВТ.</div>      <div class='cheque-row-rcolumn'>J2312</div></div>    <div cl
ass='cheque-row'>      <div class='cheque-row-lcolumn'>RRN</div>      <div class='cheque-row-rcolumn'>222223322</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>КАСИР:</div>      <div class='cheque-row-rcolumn'></div></div>    <div class='cheque-row'>      <div class='cheque-row-l
column'></div>      <div class='cheque-row-rcolumn'>..................</div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'>ДЕРЖАТЕЛЬ ЕПЗ:</div>      <div class='cheque-row-rcolumn'></div></div>    <div class='cheque-row'>      <div class='cheque-row-lcolumn'></div>      <div class='cheq
ue-row-rcolumn'>..................</div></div>  <div class='cheque-devider'>------------------------------------</div>      <div>Восток</div>      <div>Підпис власника картки не потрібний</div></div></div>  <div class='cheque-devider'>------------------------------------</div>  <div class='cheque-device-info'>
   <div class='device-info-line'>      <div class='device-info-line-item'>ФН ПРРО :</div>      <div class='device-info-line-item'>22</div></div>    <div class='device-info-line'>      <div class='device-info-line-item'>НОМЕР :</div>      <div class='device-info-line-item'>XXXS-xI</div></div>    <div
 class='device-info-line'>      <div class='device-info-line-item'>ЧАС :</div>      <div class='device-info-line-item'>14:41:35 31.12.2023</div></div>    <div class='device-info-line'>      <div class='device-info-line-item'>online :</div>      <div class='device-info-line-item'>false</div></div>    <div class=
'device-info-line'>      <div class='device-info-line-item'>hmac :</div>      <div class='device-info-line-item'>        <div>23123123213</div>        <div>2321312312</div>        <div>123123123</div>        <div>123123</div></div></div></div>  <div class='footer'>    <div class='che
que-row'>      <img class='qrcode' src='data:image/png;base64,uQmCC' /></div>    <div>      <div class='centered' style='white-space:nowrap;overflow: hidden;'>****** ФІСКАЛЬНИЙ ЧЕК ******</
div>      <div class='centered'><img style='width:100%;' src='data:image/png;base64,' /></div></div></div></div></body></html>
*/
