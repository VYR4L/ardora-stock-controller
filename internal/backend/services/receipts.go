package services

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/models"
)

const receiptHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.ReceiptIdentifier}} · Recibo não fiscal</title>
<style>
body{font-family:Arial,sans-serif;color:#171717;background:#fff;margin:0;padding:24px}main{max-width:680px;margin:auto}
h1{font-size:22px;margin:0 0 4px}.muted{color:#666}.notice{margin:18px 0;padding:10px;text-align:center;border:1px solid #bbb;font-weight:700}
table{width:100%;border-collapse:collapse;margin:18px 0}th,td{padding:8px 4px;border-bottom:1px solid #ddd;text-align:left}th:last-child,td:last-child{text-align:right}
.totals{margin-left:auto;max-width:330px}.totals div{display:flex;justify-content:space-between;padding:4px}.total{font-size:18px;font-weight:700;border-top:2px solid #222;margin-top:6px;padding-top:10px!important}
.actions{margin-top:28px}@media print{.actions{display:none}body{padding:0}}
</style>
</head>
<body><main>
<h1>{{.BusinessName}}</h1>
{{if .BusinessDocument}}<div class="muted">{{.BusinessDocument}}</div>{{end}}
{{if .BusinessAddress}}<div class="muted">{{.BusinessAddress}}</div>{{end}}
<div class="notice">RECIBO NÃO FISCAL</div>
<p><strong>{{.ReceiptIdentifier}}</strong><br>Cliente: {{.ClientName}}{{if .ClientDocument}} · {{.ClientDocument}}{{end}}<br>Entrada: {{.OpenedAt}}<br>Fechamento: {{.ClosedAt}}</p>
<table><thead><tr><th>Item</th><th>Qtd.</th><th>Unitário</th><th>Total</th></tr></thead><tbody>
<tr><td>Entrada</td><td>1</td><td>{{.EntryPrice}}</td><td>{{.EntryPrice}}</td></tr>
{{range .Lines}}<tr><td>{{.Description}}</td><td>{{.Quantity}}</td><td>{{.UnitPrice}}</td><td>{{.Total}}</td></tr>{{end}}
</tbody></table>
<div class="totals"><div><span>Consumo</span><span>{{.ConsumptionSubtotal}}</span></div><div><span>Total bruto</span><span>{{.GrossTotal}}</span></div><div><span>Desconto</span><span>- {{.Discount}}</span></div><div class="total"><span>Total</span><span>{{.FinalTotal}}</span></div></div>
<p>Pagamento: <strong>{{.PaymentMethod}}</strong><br><span class="muted">Emitido em {{.IssuedAt}}</span></p>
<div class="actions"><button onclick="window.print()">Imprimir</button></div>
</main>{{if .AutoPrint}}<script>window.addEventListener('load',function(){window.print()})</script>{{end}}</body></html>`

// ReceiptService renders and opens local, non-fiscal receipts.
type ReceiptService struct {
	visits      *VisitService
	config      *ConfigService
	receiptsDir string
	opener      func(string) error
}

func NewReceiptService(db *sql.DB, receiptsDir string) *ReceiptService {
	return &ReceiptService{
		visits:      NewVisitService(db),
		config:      NewConfigService(db),
		receiptsDir: receiptsDir,
		opener:      openLocalFile,
	}
}

// Save renders the preserved checkout snapshot and returns the HTML path.
func (s *ReceiptService) Save(ctx context.Context, visitID int64, autoPrint bool) (string, error) {
	summary, err := s.visits.Preview(ctx, visitID, 0)
	if err != nil {
		return "", err
	}
	if summary.Visit.ClosedAt == nil || summary.Visit.ReceiptIdentifier == "" {
		return "", fmt.Errorf("visit has not been checked out")
	}
	data, err := s.receiptData(ctx, summary, autoPrint)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.receiptsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.receiptsDir, summary.Visit.ReceiptIdentifier+".html")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	tpl, err := template.New("receipt").Parse(receiptHTML)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(file, data); err != nil {
		return "", err
	}
	return path, nil
}

// Open saves and opens a receipt. autoPrint asks the browser to show its print dialog.
func (s *ReceiptService) Open(ctx context.Context, visitID int64, autoPrint bool) (string, error) {
	path, err := s.Save(ctx, visitID, autoPrint)
	if err != nil {
		return "", err
	}
	if err := s.opener(path); err != nil {
		return path, err
	}
	return path, nil
}

type receiptLineData struct {
	Description string
	Quantity    int
	UnitPrice   string
	Total       string
}

type receiptData struct {
	BusinessName        string
	BusinessDocument    string
	BusinessAddress     string
	ReceiptIdentifier   string
	ClientName          string
	ClientDocument      string
	OpenedAt            string
	ClosedAt            string
	IssuedAt            string
	EntryPrice          string
	Lines               []receiptLineData
	ConsumptionSubtotal string
	GrossTotal          string
	Discount            string
	FinalTotal          string
	PaymentMethod       string
	AutoPrint           bool
}

func (s *ReceiptService) receiptData(ctx context.Context, summary *models.CheckoutSummary, autoPrint bool) (receiptData, error) {
	businessName, err := s.config.Get(ctx, "business_name")
	if err != nil {
		return receiptData{}, err
	}
	if strings.TrimSpace(businessName) == "" {
		businessName = "Ardora Luderia"
	}
	businessDocument, err := s.config.Get(ctx, "business_document")
	if err != nil {
		return receiptData{}, err
	}
	businessAddress, err := s.config.Get(ctx, "business_address")
	if err != nil {
		return receiptData{}, err
	}
	visit := summary.Visit
	data := receiptData{
		BusinessName:        businessName,
		BusinessDocument:    businessDocument,
		BusinessAddress:     businessAddress,
		ReceiptIdentifier:   visit.ReceiptIdentifier,
		ClientName:          visit.ClientName,
		OpenedAt:            visit.OpenedAt.Format("02/01/2006 15:04"),
		EntryPrice:          FormatBRLCents(visit.EntryPriceCents),
		ConsumptionSubtotal: FormatBRLCents(visit.ConsumptionSubtotalCents),
		GrossTotal:          FormatBRLCents(visit.GrossTotalCents),
		Discount:            FormatBRLCents(visit.DiscountCents),
		FinalTotal:          FormatBRLCents(visit.FinalTotalCents),
		PaymentMethod:       PaymentMethodLabel(visit.PaymentMethod),
		AutoPrint:           autoPrint,
	}
	if visit.ClientDocument != nil {
		data.ClientDocument = *visit.ClientDocument
	}
	if visit.ClosedAt != nil {
		data.ClosedAt = visit.ClosedAt.Format("02/01/2006 15:04")
	}
	if visit.ReceiptIssuedAt != nil {
		data.IssuedAt = visit.ReceiptIssuedAt.Format("02/01/2006 15:04")
	} else {
		data.IssuedAt = time.Now().Format("02/01/2006 15:04")
	}
	for _, line := range summary.Lines {
		data.Lines = append(data.Lines, receiptLineData{
			Description: line.Description,
			Quantity:    line.Quantity,
			UnitPrice:   FormatBRLCents(line.UnitPriceCents),
			Total:       FormatBRLCents(line.TotalCents),
		})
	}
	return data, nil
}

func openLocalFile(path string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path)
	case "darwin":
		command = exec.Command("open", path)
	default:
		command = exec.Command("xdg-open", path)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("open receipt: %w", err)
	}
	return nil
}
