package services

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
)

// ConfigService provides access to application settings stored in the database.
type ConfigService struct {
	db *sql.DB
}

const EntryPriceConfigKey = "entry_price_cents"

// EntryPriceCents returns the configured entry price or zero when unset.
func (s *ConfigService) EntryPriceCents(ctx context.Context) (int64, error) {
	value, err := s.Get(ctx, EntryPriceConfigKey)
	if err != nil || value == "" {
		return 0, err
	}
	cents, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cents < 0 {
		return 0, fmt.Errorf("invalid configured entry price")
	}
	return cents, nil
}

// SetEntryPriceCents validates and stores the default entry price.
func (s *ConfigService) SetEntryPriceCents(ctx context.Context, cents int64) error {
	if cents < 0 {
		return fmt.Errorf("entry price cannot be negative")
	}
	return s.Set(ctx, EntryPriceConfigKey, strconv.FormatInt(cents, 10))
}

// NewConfigService creates a new configuration service.
func NewConfigService(db *sql.DB) *ConfigService {
	return &ConfigService{db: db}
}

// Get returns the value for a given configuration key. Returns empty string if not found.
func (s *ConfigService) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM configuration WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("read config %s: %w", key, err)
	}
	return value, nil
}

// Set updates or inserts a configuration key with the given value.
func (s *ConfigService) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO configuration (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	if err != nil {
		return fmt.Errorf("write config %s: %w", key, err)
	}
	return nil
}
