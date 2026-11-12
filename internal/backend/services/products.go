package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

// ProductService handles CRUD for products.
type ProductService struct {
	db *sql.DB
}

func NewProductService(db *sql.DB) *ProductService {
	return &ProductService{db: db}
}

func (s *ProductService) List(ctx context.Context, onlyActive bool) ([]models.Product, error) {
	query := "SELECT id, name, category, price_cents, stock, min_stock, image, active, created_at, updated_at FROM products"
	if onlyActive {
		query += " WHERE active = 1"
	}
	query += " ORDER BY name ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var list []models.Product
	for rows.Next() {
		var p models.Product
		var active int
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.PriceCents, &p.Stock, &p.MinStock, &p.Image, &active, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		p.Active = active == 1
		p.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
		p.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, updated, time.Local)
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *ProductService) Create(ctx context.Context, p *models.Product) error {
	name, category, err := validateProduct(p)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if p.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO products (name, category, price_cents, stock, min_stock, image, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		name, category, p.PriceCents, p.Stock, p.MinStock, p.Image, active, now, now,
	)
	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		p.Name, p.Category = name, category
		p.ID = id
		p.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
		p.UpdatedAt = p.CreatedAt
	}
	return nil
}

func (s *ProductService) Update(ctx context.Context, p *models.Product) error {
	name, category, err := validateProduct(p)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if p.Active {
		active = 1
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE products SET name = ?, category = ?, price_cents = ?, stock = ?, min_stock = ?, image = ?, active = ?, updated_at = ? WHERE id = ?",
		name, category, p.PriceCents, p.Stock, p.MinStock, p.Image, active, now, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	if err := requireOneRow(result, "product"); err != nil {
		return err
	}
	p.Name, p.Category = name, category
	p.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	return nil
}

// UpdateCatalog updates catalog fields without overwriting intervening sales.
// expectedStock is the value loaded when the edit form opened.
func (s *ProductService) UpdateCatalog(ctx context.Context, p *models.Product, expectedStock int) error {
	name, category, err := validateProduct(p)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if p.Active {
		active = 1
	}

	var result sql.Result
	if p.Stock == expectedStock {
		result, err = s.db.ExecContext(ctx,
			"UPDATE products SET name = ?, category = ?, price_cents = ?, min_stock = ?, image = ?, active = ?, updated_at = ? WHERE id = ?",
			name, category, p.PriceCents, p.MinStock, p.Image, active, now, p.ID,
		)
	} else {
		result, err = s.db.ExecContext(ctx,
			"UPDATE products SET name = ?, category = ?, price_cents = ?, stock = ?, min_stock = ?, image = ?, active = ?, updated_at = ? WHERE id = ? AND stock = ?",
			name, category, p.PriceCents, p.Stock, p.MinStock, p.Image, active, now, p.ID, expectedStock,
		)
	}
	if err != nil {
		return fmt.Errorf("update product catalog: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("product stock changed while editing; reload and try again")
	}
	p.Name, p.Category = name, category
	p.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	return nil
}

// PlanRemoval reports whether a product can be deleted or must be archived.
func (s *ProductService) PlanRemoval(ctx context.Context, id int64) (models.RemovalPlan, error) {
	return planProductRemoval(ctx, s.db, id)
}

// Remove recomputes and applies the safe product removal policy atomically.
func (s *ProductService) Remove(ctx context.Context, id int64) (models.RemovalPlan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	defer tx.Rollback()
	plan, err := planProductRemoval(ctx, tx, id)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	var result sql.Result
	switch plan.Kind {
	case models.RemovalDelete:
		result, err = tx.ExecContext(ctx, "DELETE FROM products WHERE id = ?", id)
	case models.RemovalArchive:
		result, err = tx.ExecContext(ctx, "UPDATE products SET active = 0, updated_at = ? WHERE id = ?", time.Now().Format(database.TimeLayout), id)
	default:
		return plan, fmt.Errorf("%s", plan.Message)
	}
	if err != nil {
		return plan, fmt.Errorf("remove product: %w", err)
	}
	if err := requireOneRow(result, "product"); err != nil {
		return plan, err
	}
	if err := tx.Commit(); err != nil {
		return plan, err
	}
	return plan, nil
}

func validateProduct(p *models.Product) (string, string, error) {
	name := strings.TrimSpace(p.Name)
	category := strings.TrimSpace(p.Category)
	if name == "" {
		return "", "", fmt.Errorf("product name is required")
	}
	if p.PriceCents < 0 || p.Stock < 0 || p.MinStock < 0 {
		return "", "", fmt.Errorf("product price, stock, and minimum stock cannot be negative")
	}
	if category == "" {
		category = "Outros"
	}
	return name, category, nil
}

func planProductRemoval(ctx context.Context, query rowQuerier, id int64) (models.RemovalPlan, error) {
	var exists int
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*) FROM products WHERE id = ?", id).Scan(&exists); err != nil {
		return models.RemovalPlan{}, err
	}
	if exists != 1 {
		return models.RemovalPlan{}, fmt.Errorf("product not found")
	}
	var references int
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*) FROM history WHERE product_id = ?", id).Scan(&references); err != nil {
		return models.RemovalPlan{}, err
	}
	if references > 0 {
		return models.RemovalPlan{Kind: models.RemovalArchive, Message: "Este produto possui histórico e será arquivado."}, nil
	}
	return models.RemovalPlan{Kind: models.RemovalDelete, Message: "Este produto nunca foi utilizado e será excluído definitivamente."}, nil
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func requireOneRow(result sql.Result, entity string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%s not found", entity)
	}
	return nil
}
