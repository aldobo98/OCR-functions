package process

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

func centerY(row OCRRow) float64 {
	top := float64(row.BoundingBox[0][1])
	bottom := float64(row.BoundingBox[2][1])
	return (top + bottom) / 2
}

func centerX(row OCRRow) float64 {
	top := float64(row.BoundingBox[0][0])
	bottom := float64(row.BoundingBox[1][0])
	return (top + bottom) / 2
}

func sameLine(a, b OCRRow, tolerance int) bool {
	return math.Abs(centerY(a)-centerY(b)) <= float64(tolerance)
}

func isBetweenY(top_row, bottom_row, middle_row OCRRow) bool {
	//Az első kettő ami között, a harmadik, ami a kérdés
	top := float64(top_row.BoundingBox[3][1])
	bottom := float64(bottom_row.BoundingBox[0][1])
	middle := centerY(middle_row)
	ret := (top < middle) && (middle < bottom)
	return ret
}

func isBetweenX(left_row, right_row, middle_row OCRRow) bool {
	left := float64(left_row.BoundingBox[0][0])
	right := float64(right_row.BoundingBox[0][0])
	middle := centerX(middle_row)
	ret := (left < middle) && (middle < right)
	return ret
}

// Egy oszlopban van-e
func isAtSameX(top, bottom OCRRow, tolerance int) bool {
	center_top := centerX(top)
	return (bottom.BoundingBox[0][0]-tolerance < int(center_top)) && (int(center_top) < bottom.BoundingBox[1][0]+tolerance)
}

func createPersonFromRows(rows []OCRRow, taxId string) Person {
	var top_row OCRRow
	for _, row := range rows {
		if (top_row == OCRRow{}) {
			top_row = row
		} else if top_row.BoundingBox[0][1] > row.BoundingBox[0][1] {
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

func isBelow(top OCRRow, bottom OCRRow) bool {
	//Ha a top alsó y koordinátája felett (mert lefele nőnek a koordináták) van a bottom felső y koordinátája, akkor alatta van
	return top.BoundingBox[3][1] < bottom.BoundingBox[0][1]
}

func find_current_and_next_items(current_no, next_no int, items, summary, no OCRRow, tolerance int, data OCRResult) (OCRRow, OCRRow) {
	var current_row, next_row OCRRow
	for _, row := range data.Results {
		if isBetweenY(items, summary, row) {
			//Megkeressük a jelenlegi és a következő sort
			if isAtSameX(no, row, tolerance) {
				if (current_row == OCRRow{}) && strings.Contains(row.Text, strconv.Itoa(current_no)) {
					current_row = row
					continue
				} else if (next_row == OCRRow{}) && strings.Contains(row.Text, strconv.Itoa(next_no)) {
					next_row = row
					continue
				}
			}
		}
	}
	return current_row, next_row
}

func Process(data OCRResult) (LayoutResult, error) {
	tolerance := 10
	//Ez egy nagyon brute-force függvény lesz :'D
	if len(data.Results) == 0 {
		return LayoutResult{}, errors.New("no data was received, unable to process")
	}
	var invoice, dateOfIssue, seller, client, items, summary OCRRow
	var no, desc, qty, um, net_price OCRRow
	for _, row := range data.Results {
		if strings.Contains(row.Text, "No") {
			no = row
			continue
		}
		if strings.Contains(row.Text, "Description") {
			desc = row
			continue
		}
		if strings.Contains(strings.ToUpper(row.Text), "SUMMARY") {
			summary = row
			continue
		}
		if strings.Contains(row.Text, "Qty") {
			qty = row
			continue
		}
		if strings.Contains(row.Text, "UM") {
			um = row
			continue
		}
		if strings.Contains(row.Text, "Net price") {
			net_price = row
			continue
		}
		if strings.Contains(row.Text, "Invoice") {
			invoice = row
			continue
		}
		if strings.Contains(row.Text, "Date of issue") {
			dateOfIssue = row
			continue
		}
		if strings.Contains(row.Text, "Seller") {
			seller = row
			continue
		}
		if strings.Contains(row.Text, "Client") {
			client = row
			continue
		}
		if strings.Contains(row.Text, "ITEMS") {
			items = row
			continue
		}

	}
	var header LayoutHeader
	if (invoice != OCRRow{}) {
		inv := strings.Split(invoice.Text, ": ")
		header.No = inv[1]
	}
	for _, row := range data.Results {
		if row != dateOfIssue && sameLine(dateOfIssue, row, tolerance) {
			header.Date = row.Text
			break
		}
	}

	var l_seller LayoutSeller
	var seller_name_and_address []OCRRow
	var tax_string string
	for _, row := range data.Results {
		if isBetweenY(seller, items, row) && isBetweenX(seller, client, row) {
			if strings.Contains(row.Text, "Tax Id") {
				tax := strings.Split(row.Text, ": ")
				tax_string = tax[1]
				continue
			}
			if strings.Contains(row.Text, "IBAN") {
				iban := strings.Split(row.Text, ": ")
				l_seller.Iban = iban[1]
				continue
			}
			seller_name_and_address = append(seller_name_and_address, row)
		}
	}
	l_seller.Person = createPersonFromRows(seller_name_and_address, tax_string)
	var client_name_and_addres []OCRRow
	for _, row := range data.Results {
		if isBetweenY(client, items, row) && isAtSameX(client, row, tolerance) {
			if strings.Contains(row.Text, "Tax Id") {
				tax := strings.Split(row.Text, ": ")
				tax_string = tax[1]
				continue
			}
			client_name_and_addres = append(client_name_and_addres, row)
		}
	}
	client_person := createPersonFromRows(client_name_and_addres, tax_string)

	var net_worth_items, vat_items, gross_items OCRRow
	for _, row := range data.Results {
		if sameLine(no, row, tolerance) {
			if strings.Contains(row.Text, "Net worth") {
				net_worth_items = row
				continue
			}
			if strings.Contains(row.Text, "VAT") {
				vat_items = row
				continue
			}
			if strings.Contains(row.Text, "Gross") {
				gross_items = row
				continue
			}
		}
	}
	current_no := 1
	next_no := 2
	var current_row, next_row OCRRow
	var item_list []Item
	for {
		current_row, next_row = find_current_and_next_items(current_no, next_no, items, summary, no, tolerance, data)
		if (current_row == OCRRow{}) {
			//Nincs új sor, ki kell lépni a ciklusból
			break
		}
		if (next_row == OCRRow{}) {
			//Nincs következő sor, ez nem probléma, a summary lesz, amihez mérünk
			next_row = summary
		}

		//Elkezdjük a feldolgozást
		var item Item
		for _, row := range data.Results {
			if sameLine(current_row, row, tolerance) {
				if isAtSameX(desc, row, tolerance) {
					item.Descrition += " " + row.Text
				}
				if isAtSameX(qty, row, tolerance) {
					item.Qty = row.Text
				}
				if isAtSameX(um, row, tolerance) {
					item.UM = row.Text
				}

				if isAtSameX(net_worth_items, row, tolerance) {
					item.Net_worth = row.Text
				}
				//A net price nem közvetlen alatta van
				if isAtSameX(net_price, row, tolerance) {
					item.Net_price = row.Text
				}
				if isAtSameX(vat_items, row, tolerance) {
					item.Vat = row.Text
				}
				if isAtSameX(gross_items, row, tolerance) {
					item.Gross_worth = row.Text
				}
			}
			//A description több soros lehet
			if isBetweenX(no, qty, row) && isBetweenY(current_row, next_row, row) {
				item.Descrition += " " + row.Text
			}
		}
		//Ha befejeztük a feldolgozást, akkor lépünk egyet a sorszámokban
		item.No = current_no
		item_list = append(item_list, item)
		current_no++
		next_no++
	}

	//Össze kell gyűjteni a summary-t
	var total, vat_percent OCRRow
	for _, row := range data.Results {
		if isBelow(summary, row) {
			if strings.Contains(row.Text, "Total") {
				total = row
				continue
			}
			if strings.Contains(row.Text, "VAT [%]") {
				vat_percent = row
			}
		}
	}
	var summ Summary
	var total_money []OCRRow
	for _, row := range data.Results {
		if isBelow(summary, row) {
			if total != row && sameLine(total, row, tolerance) {
				total_money = append(total_money, row)
			}
			if isAtSameX(vat_percent, row, tolerance) {
				summ.Vat_percent = row.Text
			}
		}
	}
	sort.Slice(total_money, func(i, j int) bool {
		return total_money[i].BoundingBox[0][0] < total_money[j].BoundingBox[0][0]
	})

	summ.Net_worth = total_money[0].Text
	summ.Vat = total_money[1].Text
	summ.Gross_worth = total_money[2].Text

	return_result := LayoutResult{Header: header, Seller: l_seller, Client: client_person, Items: item_list, Summary: summ}
	return return_result, nil

}
