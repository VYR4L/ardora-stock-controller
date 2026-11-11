package services

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

// VisitService owns customer visit lifecycle, totals, and checkout.
type VisitService struct {
	db *sql.DB
}

func NewVisitService(db *sql.DB) *VisitService {
	return &VisitService{db: db}
}

// OpenVisit creates or updates the customer and assigns one available code atomically.
func (s *VisitService) OpenVisit(ctx context.Context, name string, document *string, codeValue string) (*models.Visit, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("client name is required")
	}
	canonicalCode, err := NormalizeAccessCode(codeValue)
	if err != nil {
		return nil, err
	}
	card, _ := strconv.Atoi(canonicalCode)
	if document != nil {
		trimmed := strings.TrimSpace(*document)
		if trimmed == "" {
			document = nil
		} else {
			document = &trimmed
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var accessCodeID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM access_codes WHERE value = ? AND active = 1", canonicalCode).Scan(&accessCodeID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("command code %s is unavailable", canonicalCode)
	}
	if err != nil {
		return nil, err
	}
	var codeVisitID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM visits WHERE access_code_id = ? AND closed_at IS NULL", accessCodeID).Scan(&codeVisitID)
	if err == nil {
		return nil, fmt.Errorf("command code %s is already in use", canonicalCode)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	clientID, err := findOrCreateVisitClient(ctx, tx, name, document)
	if err != nil {
		return nil, err
	}
	var activeVisitID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM visits WHERE client_id = ? AND closed_at IS NULL", clientID).Scan(&activeVisitID)
	if err == nil {
		return nil, fmt.Errorf("client already has active visit %d", activeVisitID)
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	entryPrice, err := configuredEntryPrice(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(database.TimeLayout)
	if _, err := tx.ExecContext(ctx,
		"UPDATE clients SET name = ?, document = ?, active_card = ?, checked_in_at = ?, updated_at = ? WHERE id = ?",
		name, document, card, now, now, clientID,
	); err != nil {
		return nil, fmt.Errorf("update client: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO visits (client_id, client_name, client_document, card_number, access_code_id, entry_price_cents, opened_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		clientID, name, document, card, accessCodeID, entryPrice, now,
	)
	if err != nil {
		return nil, fmt.Errorf("open visit: %w", err)
	}
	visitID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO history (kind, client_id, visit_id, description, quantity, unit_price_cents, total_cents, created_at)
		VALUES ('entry', ?, ?, 'Entrada', 1, ?, ?, ?)`,
		clientID, visitID, entryPrice, entryPrice, now,
	); err != nil {
		return nil, fmt.Errorf("record entry: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(ctx, visitID)
}

func findOrCreateVisitClient(ctx context.Context, tx *sql.Tx, name string, document *string) (int64, error) {
	var id int64
	var err error
	if document != nil {
		err = tx.QueryRowContext(ctx, "SELECT id FROM clients WHERE document = ? LIMIT 1", *document).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, "SELECT id FROM clients WHERE name = ? COLLATE NOCASE LIMIT 1", name).Scan(&id)
	}
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	now := time.Now().Format(database.TimeLayout)
	res, err := tx.ExecContext(ctx,
		"INSERT INTO clients (name, document, created_at, updated_at) VALUES (?, ?, ?, ?)",
		name, document, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert client: %w", err)
	}
	return res.LastInsertId()
}

func configuredEntryPrice(ctx context.Context, tx *sql.Tx) (int64, error) {
	var value string
	err := tx.QueryRowContext(ctx, "SELECT value FROM configuration WHERE key = ?", EntryPriceConfigKey).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cents, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cents < 0 {
		return 0, fmt.Errorf("invalid configured entry price")
	}
	return cents, nil
}

// Get loads one visit by ID.
func (s *VisitService) Get(ctx context.Context, id int64) (*models.Visit, error) {
	return scanVisit(s.db.QueryRowContext(ctx, visitSelect+" WHERE v.id = ?", id))
}

// ActiveForClient returns the customer's current visit.
func (s *VisitService) ActiveForClient(ctx context.Context, clientID int64) (*models.Visit, error) {
	return scanVisit(s.db.QueryRowContext(ctx, visitSelect+" WHERE v.client_id = ? AND v.closed_at IS NULL", clientID))
}

// ActiveForCode returns the open visit assigned to a canonical command code.
func (s *VisitService) ActiveForCode(ctx context.Context, raw string) (*models.Visit, error) {
	value, err := NormalizeAccessCode(raw)
	if err != nil {
		return nil, err
	}
	return scanVisit(s.db.QueryRowContext(ctx, visitSelect+" WHERE ac.value = ? AND v.closed_at IS NULL", value))
}

// ListActive returns every currently open visit.
func (s *VisitService) ListActive(ctx context.Context) ([]models.Visit, error) {
	return s.list(ctx, visitSelect+" WHERE v.closed_at IS NULL ORDER BY v.opened_at ASC")
}

// ListCompleted returns recently completed visits.
func (s *VisitService) ListCompleted(ctx context.Context, limit int) ([]models.Visit, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.list(ctx, visitSelect+" WHERE v.closed_at IS NOT NULL ORDER BY v.closed_at DESC LIMIT ?", limit)
}

func (s *VisitService) list(ctx context.Context, query string, args ...any) ([]models.Visit, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var visits []models.Visit
	for rows.Next() {
		visit, err := scanVisit(rows)
		if err != nil {
			return nil, err
		}
		visits = append(visits, *visit)
	}
	return visits, rows.Err()
}

// Preview calculates current checkout values without changing state.
func (s *VisitService) Preview(ctx context.Context, visitID, discountCents int64) (*models.CheckoutSummary, error) {
	visit, err := s.Get(ctx, visitID)
	if err != nil {
		return nil, err
	}
	lines, consumption, err := visitLines(ctx, s.db, visitID)
	if err != nil {
		return nil, err
	}
	if visit.ClosedAt == nil {
		gross := visit.EntryPriceCents + consumption
		if discountCents < 0 || discountCents > gross {
			return nil, fmt.Errorf("discount must be between zero and %s", FormatBRLCents(gross))
		}
		visit.ConsumptionSubtotalCents = consumption
		visit.GrossTotalCents = gross
		visit.DiscountCents = discountCents
		visit.FinalTotalCents = gross - discountCents
	}
	return &models.CheckoutSummary{Visit: *visit, Lines: lines}, nil
}

// Checkout completes a visit exactly once and returns its immutable summary.
func (s *VisitService) Checkout(ctx context.Context, visitID, discountCents int64, payment models.PaymentMethod) (*models.CheckoutSummary, error) {
	if !validPaymentMethod(payment) {
		return nil, fmt.Errorf("payment method is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	visit, err := scanVisit(tx.QueryRowContext(ctx, visitSelect+" WHERE v.id = ?", visitID))
	if err != nil {
		return nil, err
	}
	lines, consumption, err := visitLines(ctx, tx, visitID)
	if err != nil {
		return nil, err
	}
	if visit.ClosedAt != nil {
		return &models.CheckoutSummary{Visit: *visit, Lines: lines}, nil
	}
	gross := visit.EntryPriceCents + consumption
	if discountCents < 0 || discountCents > gross {
		return nil, fmt.Errorf("discount must be between zero and %s", FormatBRLCents(gross))
	}
	now := time.Now().Format(database.TimeLayout)
	receiptID := fmt.Sprintf("REC-%06d", visit.ID)
	result, err := tx.ExecContext(ctx, `
		UPDATE visits SET consumption_subtotal_cents = ?, gross_total_cents = ?, discount_cents = ?,
			final_total_cents = ?, payment_method = ?, receipt_identifier = ?, closed_at = ?, receipt_issued_at = ?
		WHERE id = ? AND closed_at IS NULL`,
		consumption, gross, discountCents, gross-discountCents, string(payment), receiptID, now, now, visit.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("close visit: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return nil, fmt.Errorf("visit is no longer open")
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE clients SET active_card = NULL, checked_in_at = NULL, updated_at = ? WHERE id = ?", now, visit.ClientID,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE tables_clients SET left_at = ? WHERE client_id = ? AND left_at IS NULL", now, visit.ClientID,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Preview(ctx, visit.ID, 0)
}

type scanner interface {
	Scan(dest ...any) error
}

const visitSelect = `SELECT v.id, v.client_id, v.client_name, v.client_document,
	v.access_code_id, COALESCE(ac.value, printf('%03d', v.card_number)), v.card_number,
	v.entry_price_cents, v.consumption_subtotal_cents, v.gross_total_cents, v.discount_cents,
	v.final_total_cents, v.payment_method, v.receipt_identifier, v.opened_at, v.closed_at, v.receipt_issued_at
	FROM visits v
	LEFT JOIN access_codes ac ON ac.id = v.access_code_id`

func scanVisit(row scanner) (*models.Visit, error) {
	var visit models.Visit
	var document, payment, receipt, opened, closed, issued sql.NullString
	var accessCodeID sql.NullInt64
	err := row.Scan(
		&visit.ID, &visit.ClientID, &visit.ClientName, &document, &accessCodeID, &visit.AccessCodeValue, &visit.CardNumber,
		&visit.EntryPriceCents, &visit.ConsumptionSubtotalCents, &visit.GrossTotalCents,
		&visit.DiscountCents, &visit.FinalTotalCents, &payment, &receipt, &opened, &closed, &issued,
	)
	if err != nil {
		return nil, err
	}
	if document.Valid {
		value := document.String
		visit.ClientDocument = &value
	}
	if accessCodeID.Valid {
		value := accessCodeID.Int64
		visit.AccessCodeID = &value
	}
	visit.PaymentMethod = models.PaymentMethod(payment.String)
	visit.ReceiptIdentifier = receipt.String
	visit.OpenedAt, _ = time.ParseInLocation(database.TimeLayout, opened.String, time.Local)
	if closed.Valid && closed.String != "" {
		value, _ := time.ParseInLocation(database.TimeLayout, closed.String, time.Local)
		visit.ClosedAt = &value
	}
	if issued.Valid && issued.String != "" {
		value, _ := time.ParseInLocation(database.TimeLayout, issued.String, time.Local)
		visit.ReceiptIssuedAt = &value
	}
	return &visit, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func visitLines(ctx context.Context, q queryer, visitID int64) ([]models.VisitLine, int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, kind, description, quantity, unit_price_cents, total_cents, created_at
		FROM history WHERE visit_id = ? AND kind = 'sale' ORDER BY created_at ASC, id ASC`, visitID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var lines []models.VisitLine
	var total int64
	for rows.Next() {
		var line models.VisitLine
		var created string
		if err := rows.Scan(&line.ID, &line.Kind, &line.Description, &line.Quantity, &line.UnitPriceCents, &line.TotalCents, &created); err != nil {
			return nil, 0, err
		}
		line.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
		total += line.TotalCents
		lines = append(lines, line)
	}
	return lines, total, rows.Err()
}

func validPaymentMethod(method models.PaymentMethod) bool {
	switch method {
	case models.PaymentCash, models.PaymentPix, models.PaymentCredit, models.PaymentDebit, models.PaymentOther:
		return true
	default:
		return false
	}
}

// PaymentMethodLabel returns the Portuguese display label used in receipts.
func PaymentMethodLabel(method models.PaymentMethod) string {
	switch method {
	case models.PaymentCash:
		return "Dinheiro"
	case models.PaymentPix:
		return "Pix"
	case models.PaymentCredit:
		return "Crédito"
	case models.PaymentDebit:
		return "Débito"
	case models.PaymentOther:
		return "Outro"
	default:
		return "Não informado"
	}
}
