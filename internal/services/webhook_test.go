package services

import "testing"

func TestParseBillPeriod(t *testing.T) {
	cases := []struct {
		name    string
		period  string
		wantYr  int
		wantMo  int
		wantErr bool
	}{
		{"claro DD-MM-YYYY", "03-03-2026", 2026, 3, false},
		{"claro con espacios", " 03-03-2026 ", 2026, 3, false},
		{"tigo YYYY-MM", "2026-03", 2026, 3, false},
		{"tigo con espacios", " 2026-03 ", 2026, 3, false},
		{"tigo mes 12", "2026-12", 2026, 12, false},
		{"vacío", "", 0, 0, true},
		{"inváldo", "2026-13", 0, 0, true},
		{"basura", "abc", 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			yr, mo, err := parseBillPeriod(c.period)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseBillPeriod(%q) se esperaba error", c.period)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBillPeriod(%q): %v", c.period, err)
			}
			if yr != c.wantYr || mo != c.wantMo {
				t.Errorf("parseBillPeriod(%q) = (%d, %d), want (%d, %d)", c.period, yr, mo, c.wantYr, c.wantMo)
			}
		})
	}
}

func TestBuildWebhookPayloadTigo(t *testing.T) {
	b := BillPayload{
		Period: "2026-03",
		Amount: "699.99",
		Status: "paid",
		Raw:    map[string]any{"invoiceId": "MSDFC-00000000"},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["year"] != 2026 || payload["month"] != 3 {
		t.Errorf("year/month = %v/%v, want 2026/3", payload["year"], payload["month"])
	}
	if payload["amount"] != 699.99 {
		t.Errorf("amount = %v, want 699.99", payload["amount"])
	}
	if payload["status"] != "paid" {
		t.Errorf("status = %v", payload["status"])
	}
	if payload["invoice_number"] != "MSDFC-00000000" {
		t.Errorf("invoice_number = %v", payload["invoice_number"])
	}
}

func TestBuildWebhookPayloadClaro(t *testing.T) {
	b := BillPayload{
		Period:    "03-03-2026",
		Amount:    "1234.56",
		Status:    "pending",
		IssueDate: "2026-03-03",
		DueDate:   "2026-04-02",
		Raw:       map[string]any{"numFactura": "INV-2026-03"},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["year"] != 2026 || payload["month"] != 3 {
		t.Errorf("year/month = %v/%v, want 2026/3", payload["year"], payload["month"])
	}
	if payload["amount"] != 1234.56 {
		t.Errorf("amount = %v, want 1234.56", payload["amount"])
	}
	if payload["invoice_number"] != "INV-2026-03" {
		t.Errorf("invoice_number = %v", payload["invoice_number"])
	}
	if payload["issue_date"] != "2026-03-03" {
		t.Errorf("issue_date = %v, want 2026-03-03", payload["issue_date"])
	}
	if payload["due_date"] != "2026-04-02" {
		t.Errorf("due_date = %v, want 2026-04-02", payload["due_date"])
	}
}

func TestBuildWebhookPayloadPeriodoInvalido(t *testing.T) {
	b := BillPayload{Period: "basura", Amount: "10", Status: "pending"}
	if _, err := buildWebhookPayload(b); err == nil {
		t.Fatal("se esperaba error con periodo inválido")
	}
}

// CA-003: DISNORTE envía issue_date = FechaRecibo y due_date = +20 días.
func TestBuildWebhookPayloadDisnorte(t *testing.T) {
	b := BillPayload{
		Period:    "2026-08",
		Amount:    "60.66",
		Status:    "paid",
		IssueDate: "2026-08-19",
		DueDate:   "2026-09-08",
		Raw:       map[string]any{"numFactura": "F122000000000001"},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["issue_date"] != "2026-08-19" {
		t.Errorf("issue_date = %v, want 2026-08-19", payload["issue_date"])
	}
	if payload["due_date"] != "2026-09-08" {
		t.Errorf("due_date = %v, want 2026-09-08 (FechaRecibo + 20 días)", payload["due_date"])
	}
	if payload["invoice_number"] != "F122000000000001" {
		t.Errorf("invoice_number = %v", payload["invoice_number"])
	}
}

// CA-002: ENACAL envía ambos campos normalizados.
func TestBuildWebhookPayloadEnacal(t *testing.T) {
	b := BillPayload{
		Period:    "2026-08",
		Amount:    "28.19",
		Status:    "pending",
		IssueDate: "2026-08-29",
		DueDate:   "2026-09-20",
		Raw:       map[string]any{"numFactura": "FAC-000000001"},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["issue_date"] != "2026-08-29" {
		t.Errorf("issue_date = %v, want 2026-08-29", payload["issue_date"])
	}
	if payload["due_date"] != "2026-09-20" {
		t.Errorf("due_date = %v, want 2026-09-20", payload["due_date"])
	}
	if payload["invoice_number"] != "FAC-000000001" {
		t.Errorf("invoice_number = %v", payload["invoice_number"])
	}
}

// CA-004: Tigo envía due_date y NO issue_date cuando falta.
func TestBuildWebhookPayloadTigoSinIssue(t *testing.T) {
	b := BillPayload{
		Period:  "2026-08",
		Amount:  "700.46",
		Status:  "pending",
		DueDate: "2026-09-19",
		Raw:     map[string]any{"invoiceId": "MSDFC-31369430"},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["due_date"] != "2026-09-19" {
		t.Errorf("due_date = %v, want 2026-09-19", payload["due_date"])
	}
	if _, ok := payload["issue_date"]; ok {
		t.Error("issue_date no debe enviarse si el plugin no lo expone")
	}
}

// CA-006: placeholders y fechas inválidas nunca se envían.
func TestBuildWebhookPayloadOmiteFechasInvalidas(t *testing.T) {
	cases := []BillPayload{
		{Period: "2026-08", Amount: "10", Status: "paid", DueDate: "0001-01-01"},
		{Period: "2026-08", Amount: "10", Status: "paid", IssueDate: "basura"},
		{Period: "2026-08", Amount: "10", Status: "paid", DueDate: "2026-08-19T00:00:00"},
	}
	for i, b := range cases {
		payload, err := buildWebhookPayload(b)
		if err != nil {
			t.Fatalf("caso %d: buildWebhookPayload: %v", i, err)
		}
		if _, ok := payload["issue_date"]; ok {
			t.Errorf("caso %d: issue_date no debe enviarse (%v)", i, payload["issue_date"])
		}
		if _, ok := payload["due_date"]; ok {
			t.Errorf("caso %d: due_date no debe enviarse (%v)", i, payload["due_date"])
		}
	}
}

// CA-007: formato inválido no rompe el job, omite el campo y sigue.
func TestBuildWebhookPayloadFechaInvalidaNoFalla(t *testing.T) {
	b := BillPayload{
		Period:    "2026-08",
		Amount:    "60.66",
		Status:    "paid",
		IssueDate: "no-es-fecha",
		DueDate:   "2026-09-08",
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if _, ok := payload["issue_date"]; ok {
		t.Error("issue_date inválido no debe enviarse")
	}
	if payload["due_date"] != "2026-09-08" {
		t.Errorf("due_date = %v, want 2026-09-08 (los demás campos siguen)", payload["due_date"])
	}
	if payload["amount"] != 60.66 {
		t.Errorf("amount = %v, want 60.66", payload["amount"])
	}
}

// tigoInvoiceIDReal es el formato real de invoiceId en la API de Mi Cuenta Tigo
// (captura real de Charles, flujo 94): objeto {label, show, value, formattedValue}.
var tigoInvoiceIDReal = map[string]any{
	"label":          "Factura:",
	"show":           true,
	"value":          "MSDFC-00000000",
	"formattedValue": "MSDFC-00000000",
}

func TestBuildWebhookPayloadTigoReal(t *testing.T) {
	b := BillPayload{
		Period: "2026-08",
		Amount: "700.46",
		Status: "pending",
		Raw:    map[string]any{"invoiceId": tigoInvoiceIDReal},
	}
	payload, err := buildWebhookPayload(b)
	if err != nil {
		t.Fatalf("buildWebhookPayload: %v", err)
	}
	if payload["invoice_number"] != "MSDFC-00000000" {
		t.Errorf("invoice_number = %q, want MSDFC-00000000 (sin map[...])", payload["invoice_number"])
	}
}

func TestExtractInvoiceNumber(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{
			"tigo objeto envuelto",
			map[string]any{"invoiceId": tigoInvoiceIDReal},
			"MSDFC-00000000",
		},
		{
			"tigo string directo",
			map[string]any{"invoiceId": "MSDFC-00000000"},
			"MSDFC-00000000",
		},
		{
			"claro numFactura string",
			map[string]any{"numFactura": "INV-2026-03"},
			"INV-2026-03",
		},
		{
			"disnorte numFactura string",
			map[string]any{"numFactura": "F122000000000001"},
			"F122000000000001",
		},
		{
			"invoice_number directo",
			map[string]any{"invoice_number": "MSDFC-00000000"},
			"MSDFC-00000000",
		},
		{
			"value compuesto cae a formattedValue",
			map[string]any{"invoiceId": map[string]any{
				"value":          map[string]any{"a": 1},
				"formattedValue": "MSDFC-00000000",
			}},
			"MSDFC-00000000",
		},
		{
			"sin número",
			map[string]any{"period": "2026-08"},
			"",
		},
		{
			"nil",
			nil,
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractInvoiceNumber(c.raw); got != c.want {
				t.Errorf("extractInvoiceNumber() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestScalarValue(t *testing.T) {
	if got := scalarValue(700.46); got != "700.46" {
		t.Errorf("float = %q, want 700.46", got)
	}
	if got := scalarValue(700); got != "700" {
		t.Errorf("int = %q, want 700", got)
	}
	if got := scalarValue(" MSDFC-00000000 "); got != "MSDFC-00000000" {
		t.Errorf("string = %q", got)
	}
	if got := scalarValue(nil); got != "" {
		t.Errorf("nil = %q, want vacío", got)
	}
}
