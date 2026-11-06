package services

import (
	"context"
	"database/sql"
	"fmt"
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
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if p.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO products (name, category, price_cents, stock, min_stock, image, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		p.Name, p.Category, p.PriceCents, p.Stock, p.MinStock, p.Image, active, now, now,
	)
	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		p.ID = id
		p.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
		p.UpdatedAt = p.CreatedAt
	}
	return nil
}

func (s *ProductService) Update(ctx context.Context, p *models.Product) error {
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if p.Active {
		active = 1
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE products SET name = ?, category = ?, price_cents = ?, stock = ?, min_stock = ?, image = ?, active = ?, updated_at = ? WHERE id = ?",
		p.Name, p.Category, p.PriceCents, p.Stock, p.MinStock, p.Image, active, now, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	p.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	return nil
}
