package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"

	"github.com/ardora/stockcontroller/internal/backend/database"
)

const accessCodeSheetHTML = `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Códigos Ardora · Lote {{.BatchID}}</title>
<style>
@page{size:A4 portrait;margin:10mm}*{box-sizing:border-box}body{margin:0;background:#fff;color:#111;font-family:Arial,sans-serif}
.sheet{width:190mm;height:277mm;display:grid;grid-template-columns:repeat(3,1fr);grid-template-rows:repeat(4,1fr);gap:4mm;page-break-after:always}.sheet:last-child{page-break-after:auto}
.label{border:1px dashed #888;border-radius:3mm;padding:3mm;display:flex;flex-direction:column;align-items:center;justify-content:center;overflow:hidden}.brand{font-size:10pt;font-weight:700;letter-spacing:.12em}.number{font-size:25pt;font-weight:800;line-height:1;margin:1mm 0}.qr{width:26mm;height:26mm;image-rendering:pixelated}.barcode{width:50mm;height:12mm;object-fit:fill;margin-top:1.5mm}.value{font-family:monospace;font-size:10pt;margin-top:.5mm}.actions{position:fixed;right:12px;bottom:12px}@media print{.actions{display:none}}
</style></head><body>
{{range .Sheets}}<section class="sheet">{{range .}}<article class="label"><div class="brand">ARDORA</div><div class="number">{{.Value}}</div><img class="qr" src="{{.QR}}" alt="QR {{.Value}}"><img class="barcode" src="{{.Code128}}" alt="Código de barras {{.Value}}"><div class="value">{{.Value}}</div></article>{{end}}</section>{{end}}
<button class="actions" onclick="window.print()">Imprimir</button>{{if .AutoPrint}}<script>window.addEventListener('load',function(){window.print()})</script>{{end}}
</body></html>`

type AccessCodePrintService struct {
	db        *sql.DB
	codes     *AccessCodeService
	sheetsDir string
	opener    func(string) error
}

func NewAccessCodePrintService(db *sql.DB, sheetsDir string) *AccessCodePrintService {
	return &AccessCodePrintService{
		db:        db,
		codes:     NewAccessCodeService(db),
		sheetsDir: sheetsDir,
		opener:    openLocalFile,
	}
}

type accessCodeLabel struct {
	Value   string
	QR      template.URL
	Code128 template.URL
}

type accessCodeSheetData struct {
	BatchID   int64
	Sheets    [][]accessCodeLabel
	AutoPrint bool
}

// SaveBatch renders one immutable batch to a deterministic local HTML file.
func (s *AccessCodePrintService) SaveBatch(ctx context.Context, batchID int64, autoPrint bool) (string, error) {
	codes, err := s.codes.ListBatchCodes(ctx, batchID)
	if err != nil {
		return "", err
	}
	if len(codes) == 0 {
		return "", fmt.Errorf("access code batch %d has no codes", batchID)
	}
	labels := make([]accessCodeLabel, 0, len(codes))
	for _, code := range codes {
		qrCode, barCode, err := encodeAccessCodeBarcodes(code.Value)
		if err != nil {
			return "", err
		}
		qrData, err := barcodeDataURI(qrCode)
		if err != nil {
			return "", err
		}
		barData, err := barcodeDataURI(barCode)
		if err != nil {
			return "", err
		}
		labels = append(labels, accessCodeLabel{Value: code.Value, QR: template.URL(qrData), Code128: template.URL(barData)})
	}
	sheets := make([][]accessCodeLabel, 0, (len(labels)+11)/12)
	for start := 0; start < len(labels); start += 12 {
		end := start + 12
		if end > len(labels) {
			end = len(labels)
		}
		sheets = append(sheets, labels[start:end])
	}
	tpl, err := template.New("access-code-sheet").Parse(accessCodeSheetHTML)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, accessCodeSheetData{BatchID: batchID, Sheets: sheets, AutoPrint: autoPrint}); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.sheetsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.sheetsDir, fmt.Sprintf("codigos-lote-%06d.html", batchID))
	if err := os.WriteFile(path, rendered.Bytes(), 0o644); err != nil {
		return "", err
	}
	now := time.Now().Format(database.TimeLayout)
	result, err := s.db.ExecContext(ctx, "UPDATE access_code_batches SET last_printed_at = ? WHERE id = ?", now, batchID)
	if err != nil {
		return path, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return path, fmt.Errorf("access code batch %d not found", batchID)
	}
	return path, nil
}

// OpenBatch saves a sheet and delegates display/printing to the OS.
func (s *AccessCodePrintService) OpenBatch(ctx context.Context, batchID int64, autoPrint bool) (string, error) {
	path, err := s.SaveBatch(ctx, batchID, autoPrint)
	if err != nil {
		return path, err
	}
	if err := s.opener(path); err != nil {
		return path, err
	}
	return path, nil
}

func encodeAccessCodeBarcodes(value string) (barcode.Barcode, barcode.Barcode, error) {
	encodedQR, err := qr.Encode(value, qr.M, qr.Auto)
	if err != nil {
		return nil, nil, err
	}
	qrCode, err := barcode.Scale(encodedQR, 144, 144)
	if err != nil {
		return nil, nil, err
	}
	encodedBar, err := code128.Encode(value)
	if err != nil {
		return nil, nil, err
	}
	barCode, err := barcode.Scale(encodedBar, 360, 80)
	if err != nil {
		return nil, nil, err
	}
	return qrCode, barCode, nil
}

func barcodeDataURI(code barcode.Barcode) (string, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, code); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}
