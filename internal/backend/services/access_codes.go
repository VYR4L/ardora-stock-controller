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

// AccessCodeService owns reusable command-code generation and lifecycle.
type AccessCodeService struct {
	db *sql.DB
}

func NewAccessCodeService(db *sql.DB) *AccessCodeService {
	return &AccessCodeService{db: db}
}

// NormalizeAccessCode converts scanner or manual input to its canonical value.
func NormalizeAccessCode(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 3 {
		return "", fmt.Errorf("command code must be numeric between 000 and 999")
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("command code must be numeric between 000 and 999")
		}
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 || number > 999 {
		return "", fmt.Errorf("command code must be numeric between 000 and 999")
	}
	return fmt.Sprintf("%03d", number), nil
}

// GenerateBatch creates one contiguous code range atomically.
func (s *AccessCodeService) GenerateBatch(ctx context.Context, quantity int) (*models.AccessCodeBatch, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var maximum int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(number), 0) FROM access_codes").Scan(&maximum); err != nil {
		return nil, err
	}
	first := maximum + 1
	last := first + quantity - 1
	if first < 1 || last > 999 {
		return nil, fmt.Errorf("requested batch exceeds command code 999")
	}
	now := time.Now().Format(database.TimeLayout)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO access_code_batches(first_number, last_number, quantity, legacy, created_at)
		VALUES(?, ?, ?, 0, ?)`, first, last, quantity, now)
	if err != nil {
		return nil, fmt.Errorf("create access code batch: %w", err)
	}
	batchID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	for number := first; number <= last; number++ {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO access_codes(batch_id, number, value, active, created_at)
			VALUES(?, ?, ?, 1, ?)`, batchID, number, fmt.Sprintf("%03d", number), now); err != nil {
			return nil, fmt.Errorf("create access code %03d: %w", number, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	createdAt, _ := time.ParseInLocation(database.TimeLayout, now, time.Local)
	return &models.AccessCodeBatch{
		ID:          batchID,
		FirstNumber: first,
		LastNumber:  last,
		Quantity:    quantity,
		CreatedAt:   createdAt,
	}, nil
}

// ListBatchCodes returns codes from one immutable batch in numeric order.
func (s *AccessCodeService) ListBatchCodes(ctx context.Context, batchID int64) ([]models.AccessCode, error) {
	return s.listCodes(ctx, `WHERE ac.batch_id = ? ORDER BY ac.number`, batchID)
}

// ListBatches returns newest generated batches first.
func (s *AccessCodeService) ListBatches(ctx context.Context) ([]models.AccessCodeBatch, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, first_number, last_number, quantity, legacy, created_at, last_printed_at
		FROM access_code_batches ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var batches []models.AccessCodeBatch
	for rows.Next() {
		var batch models.AccessCodeBatch
		var legacy int
		var created string
		var printed sql.NullString
		if err := rows.Scan(&batch.ID, &batch.FirstNumber, &batch.LastNumber, &batch.Quantity, &legacy, &created, &printed); err != nil {
			return nil, err
		}
		batch.Legacy = legacy == 1
		batch.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
		if printed.Valid && printed.String != "" {
			value, _ := time.ParseInLocation(database.TimeLayout, printed.String, time.Local)
			batch.LastPrintedAt = &value
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

// ListCodes returns all codes, or the canonical numeric match for query.
func (s *AccessCodeService) ListCodes(ctx context.Context, query string) ([]models.AccessCode, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.listCodes(ctx, "ORDER BY ac.number")
	}
	canonical, err := NormalizeAccessCode(query)
	if err != nil {
		return []models.AccessCode{}, nil
	}
	return s.listCodes(ctx, "WHERE ac.value = ? ORDER BY ac.number", canonical)
}

// Stats returns state totals and the next sequential number.
func (s *AccessCodeService) Stats(ctx context.Context) (models.AccessCodeStats, error) {
	var stats models.AccessCodeStats
	var maximum int
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN ac.active = 1 AND NOT EXISTS(
				SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL
			) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ac.active = 1 AND EXISTS(
				SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL
			) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN ac.active = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(MAX(ac.number), 0)
		FROM access_codes ac`).Scan(&stats.Total, &stats.Available, &stats.InUse, &stats.Archived, &maximum)
	if err != nil {
		return models.AccessCodeStats{}, err
	}
	stats.NextNumber = maximum + 1
	if stats.NextNumber < 1 {
		stats.NextNumber = 1
	}
	return stats, nil
}

// Resolve loads a code regardless of active state.
func (s *AccessCodeService) Resolve(ctx context.Context, raw string) (*models.AccessCode, error) {
	value, err := NormalizeAccessCode(raw)
	if err != nil {
		return nil, err
	}
	return scanAccessCode(s.db.QueryRowContext(ctx, `
		SELECT ac.id, ac.batch_id, ac.number, ac.value, ac.active, ac.created_at, ac.archived_at,
			EXISTS(SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL)
		FROM access_codes ac WHERE ac.value = ?`, value))
}

// ResolveAvailable loads an active code that is not assigned to an open visit.
func (s *AccessCodeService) ResolveAvailable(ctx context.Context, raw string) (*models.AccessCode, error) {
	code, err := s.Resolve(ctx, raw)
	if err != nil {
		return nil, err
	}
	switch code.State {
	case models.AccessCodeArchived:
		return nil, fmt.Errorf("command code %s is archived", code.Value)
	case models.AccessCodeInUse:
		return nil, fmt.Errorf("command code %s is already in use", code.Value)
	default:
		return code, nil
	}
}

// Archive removes an unused code from future check-ins.
func (s *AccessCodeService) Archive(ctx context.Context, codeID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active, inUse int
	if err := tx.QueryRowContext(ctx, `
		SELECT active, EXISTS(
			SELECT 1 FROM visits WHERE access_code_id = access_codes.id AND closed_at IS NULL
		) FROM access_codes WHERE id = ?`, codeID).Scan(&active, &inUse); err != nil {
		return err
	}
	if inUse == 1 {
		return fmt.Errorf("command code is in use")
	}
	if active == 0 {
		return tx.Commit()
	}
	now := time.Now().Format(database.TimeLayout)
	if _, err := tx.ExecContext(ctx, "UPDATE access_codes SET active = 0, archived_at = ? WHERE id = ?", now, codeID); err != nil {
		return err
	}
	return tx.Commit()
}

// Reactivate makes an archived, unused code available again.
func (s *AccessCodeService) Reactivate(ctx context.Context, codeID int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE access_codes SET active = 1, archived_at = NULL WHERE id = ?", codeID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *AccessCodeService) listCodes(ctx context.Context, suffix string, args ...any) ([]models.AccessCode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ac.id, ac.batch_id, ac.number, ac.value, ac.active, ac.created_at, ac.archived_at,
			EXISTS(SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL)
		FROM access_codes ac `+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codes []models.AccessCode
	for rows.Next() {
		code, err := scanAccessCode(rows)
		if err != nil {
			return nil, err
		}
		codes = append(codes, *code)
	}
	return codes, rows.Err()
}

func scanAccessCode(row scanner) (*models.AccessCode, error) {
	var code models.AccessCode
	var active, inUse int
	var created string
	var archived sql.NullString
	if err := row.Scan(&code.ID, &code.BatchID, &code.Number, &code.Value, &active, &created, &archived, &inUse); err != nil {
		return nil, err
	}
	code.Active = active == 1
	code.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
	if archived.Valid && archived.String != "" {
		value, _ := time.ParseInLocation(database.TimeLayout, archived.String, time.Local)
		code.ArchivedAt = &value
	}
	switch {
	case !code.Active:
		code.State = models.AccessCodeArchived
	case inUse == 1:
		code.State = models.AccessCodeInUse
	default:
		code.State = models.AccessCodeAvailable
	}
	return &code, nil
}
