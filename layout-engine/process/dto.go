package process

import "encoding/json"

type OCRRow struct {
	BoundingBox [4][2]int `json:"bounding_box"`
	Text        string    `json:"text"`
	Confidence  float64   `json:"confidence"`
}

type OCRResult struct {
	Results []OCRRow `json:"results"`
}

type LayoutResult struct {
	Header  LayoutHeader `json:"header"`
	Seller  LayoutSeller `json:"seller"`
	Client  Person       `json:"client"`
	Items   []Item       `json:"items"`
	Summary Summary      `json:"summary"`
}

type LayoutHeader struct {
	No   string `json:"invoice_no"`
	Date string `json:"date_of_issue"`
}

type LayoutSeller struct {
	Person
	Iban string `json:"iban"`
}

type Person struct {
	Name    string   `json:"name"`
	Address []string `json:"address"` //Multiple lines of address
	TaxId   string   `json:"tax_id"`
}

type Item struct {
	No          int    `json:"item_no"`
	Descrition  string `json:"item_description"`
	Qty         string `json:"item_qty"`
	UM          string `json:"item_um"`
	Net_price   string `json:"item_net_price"`
	Net_worth   string `json:"item_net_worth"`
	Vat         string `json:"item_vat"`
	Gross_worth string `json:"item_gross_worth"`
}

type Summary struct {
	Net_worth   string `json:"net_worth"`
	Vat         string `json:"vat"`
	Gross_worth string `json:"gross_worth"`
	Vat_percent string `json:"vat_percent"`
}

type StartLayoutAnalysis struct {
	JobID       string    `json:"jobId"`
	OcrData     OCRResult `json:"ocr_data"`
	CallBackUrl string    `json:"callbackUrl"`
}

type Callback struct {
	JobID        string          `json:"jobId"`
	Step         string          `json:"step"`
	Status       string          `json:"status"`
	Data         json.RawMessage `json:"data"`
	ErrorMessage string          `json:"error"`
}
