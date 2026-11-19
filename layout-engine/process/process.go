package process

import (
	"errors"
	"math"
	"sort"
	"strings"
)

func centerY(row OCRRow) float64 {
	top := float64(row.BoundingBox[0][1])
	bottom := float64(row.BoundingBox[2][1])
	return (top + bottom) / 2
}

func centerX(row OCRRow) float64 {
	left := float64(row.BoundingBox[0][0])
	right := float64(row.BoundingBox[1][0])
	return (left + right) / 2
}

func sameLine(a, b OCRRow, tolerance int) bool {
	return math.Abs(centerY(a)-centerY(b)) <= float64(tolerance)
}

func isBetweenY(top_row, bottom_row, middle_row OCRRow) bool {
	//Az első kettő ami között, a harmadik, ami a kérdés
	top := float64(top_row.BoundingBox[3][1])
	bottom := float64(bottom_row.BoundingBox[0][1])
	middle := centerY(middle_row)
	return (top < middle) && (middle < bottom)
}

func isBetweenX(left_row, right_row, middle_row OCRRow) bool {
	left := float64(left_row.BoundingBox[0][0])
	right := float64(right_row.BoundingBox[0][0])
	middle := centerX(middle_row)
	return (left < middle) && (middle < right)
}

// Egy oszlopban van-e
func isAtSameX(top, bottom OCRRow, tolerance int) bool {
	center_top := centerX(top)
	return (bottom.BoundingBox[0][0]-tolerance < int(center_top)) && (int(center_top) < bottom.BoundingBox[1][0]+tolerance)
}

// Vízszintesen átfed-e a sor a fejléccel (több darabra tört cellákhoz)
func overlapsX(header, row OCRRow, tolerance int) bool {
	return row.BoundingBox[0][0] < header.BoundingBox[1][0]+tolerance &&
		row.BoundingBox[1][0] > header.BoundingBox[0][0]-tolerance
}

func isBelow(top OCRRow, bottom OCRRow) bool {
	//Ha a top alsó y koordinátája felett (mert lefele nőnek a koordináták) van a bottom felső y koordinátája, akkor alatta van
	return top.BoundingBox[3][1] < bottom.BoundingBox[0][1]
}

func isEmpty(row OCRRow) bool {
	return row == OCRRow{}
}

// Címke normalizálása: szóközök, a végén lévő '.' és ':' levágása
func normalizeLabel(text string) string {
	t := strings.TrimSpace(text)
	t = strings.TrimRight(t, ".:")
	return strings.TrimSpace(t)
}

// Pontos (kis-nagybetű független) egyezés egy címkével, hogy pl. a "Nordic" ne legyen "No"
func isLabel(text, label string) bool {
	return strings.EqualFold(normalizeLabel(text), label)
}

func hasPrefixFold(text, prefix string) bool {
	t := strings.TrimSpace(text)
	return len(t) >= len(prefix) && strings.EqualFold(t[:len(prefix)], prefix)
}

// A kettőspont utáni rész, pl. "Tax Id: 123" -> "123"
func valueAfterColon(text string) string {
	idx := strings.Index(text, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(text[idx+1:])
}

// Sorok szövegének összefűzése balról jobbra
func joinByX(rows []OCRRow) string {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].BoundingBox[0][0] < rows[j].BoundingBox[0][0]
	})
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		parts = append(parts, strings.TrimSpace(r.Text))
	}
	return strings.Join(parts, " ")
}

func createPersonFromRows(rows []OCRRow, taxId string) Person {
	var top_row OCRRow
	for _, row := range rows {
		if isEmpty(top_row) || top_row.BoundingBox[0][1] > row.BoundingBox[0][1] {
			top_row = row
		}
	}
	var address []string
	for _, row := range rows {
		if row != top_row {
			address = append(address, row.Text)
		}
	}
	return Person{Name: top_row.Text, Address: address, TaxId: taxId}
}

// Csak akkor állítja be, ha még nincs érték (az első találat nyer)
func setFirst(target *OCRRow, row OCRRow) {
	if isEmpty(*target) {
		*target = row
	}
}

func Process(data OCRResult) (LayoutResult, error) {
	tolerance := 10
	if len(data.Results) == 0 {
		return LayoutResult{}, errors.New("no data was received, unable to process")
	}

	// 1. kör: a fő szekciócímkék
	var invoice, dateOfIssue, seller, client, items, summary OCRRow
	for _, row := range data.Results {
		switch {
		case hasPrefixFold(row.Text, "Invoice no"):
			setFirst(&invoice, row)
		case isLabel(row.Text, "Date of issue"):
			setFirst(&dateOfIssue, row)
		case isLabel(row.Text, "Seller"):
			setFirst(&seller, row)
		case isLabel(row.Text, "Client"):
			setFirst(&client, row)
		case isLabel(row.Text, "Items"):
			setFirst(&items, row)
		case isLabel(row.Text, "Summary"):
			setFirst(&summary, row)
		}
	}
	if isEmpty(items) || isEmpty(summary) {
		return LayoutResult{}, errors.New("ITEMS or SUMMARY section not found")
	}

	// 2. kör: a tételtábla fejlécei, csak az ITEMS és SUMMARY között
	var no, desc, qty, um, net_price OCRRow
	for _, row := range data.Results {
		if !isBetweenY(items, summary, row) {
			continue
		}
		switch {
		case isLabel(row.Text, "No"):
			setFirst(&no, row)
		case isLabel(row.Text, "Description"):
			setFirst(&desc, row)
		case isLabel(row.Text, "Qty"):
			setFirst(&qty, row)
		case isLabel(row.Text, "UM"):
			setFirst(&um, row)
		case isLabel(row.Text, "Net price"):
			setFirst(&net_price, row)
		}
	}
	if isEmpty(no) || isEmpty(qty) {
		return LayoutResult{}, errors.New("item table header (No / Qty) not found")
	}

	var net_worth_items, vat_items, gross_items OCRRow
	for _, row := range data.Results {
		if !sameLine(no, row, tolerance) {
			continue
		}
		switch {
		case isLabel(row.Text, "Net worth"):
			setFirst(&net_worth_items, row)
		case hasPrefixFold(row.Text, "VAT"):
			setFirst(&vat_items, row)
		case hasPrefixFold(row.Text, "Gross"):
			setFirst(&gross_items, row)
		}
	}

	// Fejléc
	var header LayoutHeader
	if !isEmpty(invoice) {
		header.No = valueAfterColon(invoice.Text)
	}
	if !isEmpty(dateOfIssue) {
		for _, row := range data.Results {
			if row != dateOfIssue && sameLine(dateOfIssue, row, tolerance) {
				header.Date = row.Text
				break
			}
		}
	}

	// Eladó
	var l_seller LayoutSeller
	if !isEmpty(seller) && !isEmpty(client) {
		var seller_rows []OCRRow
		var seller_tax string
		for _, row := range data.Results {
			if isBetweenY(seller, items, row) && isBetweenX(seller, client, row) {
				if strings.Contains(row.Text, "Tax Id") {
					seller_tax = valueAfterColon(row.Text)
					continue
				}
				if strings.Contains(row.Text, "IBAN") {
					l_seller.Iban = valueAfterColon(row.Text)
					continue
				}
				seller_rows = append(seller_rows, row)
			}
		}
		l_seller.Person = createPersonFromRows(seller_rows, seller_tax)
	}

	// Vevő
	var client_person Person
	if !isEmpty(client) {
		var client_rows []OCRRow
		var client_tax string
		for _, row := range data.Results {
			if isBetweenY(client, items, row) && isAtSameX(client, row, tolerance) {
				if strings.Contains(row.Text, "Tax Id") {
					client_tax = valueAfterColon(row.Text)
					continue
				}
				client_rows = append(client_rows, row)
			}
		}
		client_person = createPersonFromRows(client_rows, client_tax)
	}

	// Tételsorok horgonyai: a Qty oszlop értékei (minden tételnek van mennyisége,
	// az OCR-ezett sorszám viszont hiányozhat vagy torzulhat, pl. "3_")
	var anchors []OCRRow
	for _, row := range data.Results {
		if isBelow(qty, row) && isBetweenY(items, summary, row) && overlapsX(qty, row, tolerance) {
			anchors = append(anchors, row)
		}
	}
	sort.Slice(anchors, func(i, j int) bool { return centerY(anchors[i]) < centerY(anchors[j]) })
	// Egy sorban lévő, több darabra tört mennyiség ne legyen külön tétel
	var item_anchors []OCRRow
	for _, a := range anchors {
		if len(item_anchors) > 0 && sameLine(item_anchors[len(item_anchors)-1], a, tolerance) {
			continue
		}
		item_anchors = append(item_anchors, a)
	}

	// Egy oszlop cellája az adott tételsorban: minden doboz, ami a sorban van és átfed a fejléccel
	cell := func(anchor, column OCRRow) string {
		if isEmpty(column) {
			return ""
		}
		var parts []OCRRow
		for _, row := range data.Results {
			if row != column && sameLine(anchor, row, tolerance) && overlapsX(column, row, tolerance) {
				parts = append(parts, row)
			}
		}
		return joinByX(parts)
	}

	var item_list []Item
	for i, anchor := range item_anchors {
		// A tétel a horgony tetejétől a következő horgony tetejéig (vagy a SUMMARY-ig) tart
		top := float64(anchor.BoundingBox[0][1] - tolerance)
		var bottom float64
		if i+1 < len(item_anchors) {
			bottom = float64(item_anchors[i+1].BoundingBox[0][1] - tolerance)
		} else {
			bottom = float64(summary.BoundingBox[0][1])
		}

		// A leírás több soros lehet: a No és a Qty oszlop közötti összes doboz
		var desc_rows []OCRRow
		for _, row := range data.Results {
			cy, cx := centerY(row), centerX(row)
			if cy >= top && cy < bottom &&
				cx > float64(no.BoundingBox[1][0]) && cx < float64(qty.BoundingBox[0][0]) {
				desc_rows = append(desc_rows, row)
			}
		}
		sort.Slice(desc_rows, func(a, b int) bool {
			if sameLine(desc_rows[a], desc_rows[b], tolerance) {
				return desc_rows[a].BoundingBox[0][0] < desc_rows[b].BoundingBox[0][0]
			}
			return centerY(desc_rows[a]) < centerY(desc_rows[b])
		})
		var desc_parts []string
		for _, r := range desc_rows {
			desc_parts = append(desc_parts, strings.TrimSpace(r.Text))
		}

		item_list = append(item_list, Item{
			No:          i + 1,
			Descrition:  strings.Join(desc_parts, " "),
			Qty:         cell(anchor, qty),
			UM:          cell(anchor, um),
			Net_price:   cell(anchor, net_price),
			Net_worth:   cell(anchor, net_worth_items),
			Vat:         cell(anchor, vat_items),
			Gross_worth: cell(anchor, gross_items),
		})
	}

	// Összesítő
	var total, vat_percent OCRRow
	for _, row := range data.Results {
		if !isBelow(summary, row) {
			continue
		}
		switch {
		case isLabel(row.Text, "Total"):
			setFirst(&total, row)
		case isLabel(row.Text, "VAT [%]"):
			setFirst(&vat_percent, row)
		}
	}
	var summ Summary
	if !isEmpty(vat_percent) {
		for _, row := range data.Results {
			if isBelow(vat_percent, row) && isAtSameX(vat_percent, row, tolerance) {
				summ.Vat_percent = row.Text
				break
			}
		}
	}
	if !isEmpty(total) {
		var total_money []OCRRow
		for _, row := range data.Results {
			if row != total && isBelow(summary, row) && sameLine(total, row, tolerance) {
				total_money = append(total_money, row)
			}
		}
		sort.Slice(total_money, func(i, j int) bool {
			return total_money[i].BoundingBox[0][0] < total_money[j].BoundingBox[0][0]
		})
		if len(total_money) < 3 {
			return LayoutResult{}, errors.New("could not find net worth, VAT and gross worth in the Total row")
		}
		summ.Net_worth = total_money[0].Text
		summ.Vat = total_money[1].Text
		summ.Gross_worth = total_money[2].Text
	}

	return LayoutResult{Header: header, Seller: l_seller, Client: client_person, Items: item_list, Summary: summ}, nil
}
