package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

// SalesService handles clients and billing.
type SalesService struct {
	db *sql.DB
}

func NewSalesService(db *sql.DB) *SalesService {
	return &SalesService{db: db}
}

// RegisterClient creates or updates a client and assigns them an active card (0-999).
func (s *SalesService) RegisterClient(ctx context.Context, name string, document *string, card int) (*models.Client, error) {
	visit, err := NewVisitService(s.db).OpenVisit(ctx, name, document, card)
	if err != nil {
		return nil, err
	}
	return s.GetClient(ctx, visit.ClientID)
}

func (s *SalesService) GetClient(ctx context.Context, id int64) (*models.Client, error) {
	var c models.Client
	var doc sql.NullString
	var card sql.NullInt32
	var checkedIn, created, updated string

	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, document, active_card, checked_in_at, created_at, updated_at FROM clients WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Name, &doc, &card, &checkedIn, &created, &updated)

	if err != nil {
		return nil, err
	}

	if doc.Valid {
		d := doc.String
		c.Document = &d
	}
	if card.Valid {
		cv := int(card.Int32)
		c.ActiveCard = &cv
	}
	if checkedIn != "" {
		t, _ := time.ParseInLocation(database.TimeLayout, checkedIn, time.Local)
		c.CheckedInAt = &t
	}
	c.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
	c.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, updated, time.Local)

	return &c, nil
}

// BillProduct bills a product to a client.
func (s *SalesService) BillProduct(ctx context.Context, clientID, productID int64, qty int) error {
	if qty <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var visitID int64
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM visits WHERE client_id = ? AND closed_at IS NULL", clientID,
	).Scan(&visitID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("client has no active visit")
		}
		return err
	}

	// Get product details
	var productName string
	var priceCents int64
	var stock int
	err = tx.QueryRowContext(ctx, "SELECT name, price_cents, stock FROM products WHERE id = ? AND active = 1", productID).Scan(&productName, &priceCents, &stock)
	if err != nil {
		return fmt.Errorf("get product: %w", err)
	}

	if stock < qty {
		return fmt.Errorf("insufficient stock")
	}

	now := time.Now().Format(database.TimeLayout)
	totalCents := priceCents * int64(qty)

	// Insert history
	_, err = tx.ExecContext(ctx,
		"INSERT INTO history (kind, client_id, product_id, visit_id, description, quantity, unit_price_cents, total_cents, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"sale", clientID, productID, visitID, productName, qty, priceCents, totalCents, now,
	)
	if err != nil {
		return fmt.Errorf("insert history: %w", err)
	}

	// Update stock
	_, err = tx.ExecContext(ctx, "UPDATE products SET stock = stock - ?, updated_at = ? WHERE id = ?", qty, now, productID)
	if err != nil {
		return fmt.Errorf("update stock: %w", err)
	}

	return tx.Commit()
}

// Checkout removes the active card from the client, meaning they left.
func (s *SalesService) Checkout(ctx context.Context, clientID int64) error {
	visit, err := NewVisitService(s.db).ActiveForClient(ctx, clientID)
	if err != nil {
		return err
	}
	_, err = NewVisitService(s.db).Checkout(ctx, visit.ID, 0, models.PaymentOther)
	return err
}
