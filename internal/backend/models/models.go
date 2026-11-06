package models

import "time"

// Client represents a customer in the system.
type Client struct {
	ID          int64
	Name        string
	Document    *string
	ActiveCard  *int
	CheckedInAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Product represents an item or consumable to be sold.
type Product struct {
	ID         int64
	Name       string
	Category   string
	PriceCents int64
	Stock      int
	MinStock   int
	Image      string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Game represents a board game in the catalog.
type Game struct {
	ID         int64
	Name       string
	MinPlayers int
	MaxPlayers int
	Notes      string
	Image      string
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Table represents a physical table where games are played.
type Table struct {
	ID        int64
	Name      string
	GameID    *int64 // ID of the game currently being played, if any
	Active    bool
	CreatedAt time.Time
}

// PaymentMethod identifies how a completed visit was paid.
type PaymentMethod string

const (
	PaymentCash   PaymentMethod = "cash"
	PaymentPix    PaymentMethod = "pix"
	PaymentCredit PaymentMethod = "credit"
	PaymentDebit  PaymentMethod = "debit"
	PaymentOther  PaymentMethod = "other"
)

// Visit is one customer's attendance and financial snapshot.
type Visit struct {
	ID                       int64
	ClientID                 int64
	ClientName               string
	ClientDocument           *string
	CardNumber               int
	EntryPriceCents          int64
	ConsumptionSubtotalCents int64
	GrossTotalCents          int64
	DiscountCents            int64
	FinalTotalCents          int64
	PaymentMethod            PaymentMethod
	ReceiptIdentifier        string
	OpenedAt                 time.Time
	ClosedAt                 *time.Time
	ReceiptIssuedAt          *time.Time
}

// VisitLine is an immutable charge included in a visit.
type VisitLine struct {
	ID             int64
	Kind           string
	Description    string
	Quantity       int
	UnitPriceCents int64
	TotalCents     int64
	CreatedAt      time.Time
}

// CheckoutSummary is the itemized view used for preview, persistence, and receipts.
type CheckoutSummary struct {
	Visit Visit
	Lines []VisitLine
}

// TableGameSession records one time-bounded game played at a table.
type TableGameSession struct {
	ID                int64
	TableID           int64
	GameID            *int64
	GameName          string
	BroughtGameName   string
	BroughtByClientID *int64
	BroughtByName     string
	StartedAt         time.Time
	EndedAt           *time.Time
}

// TableOverview combines a physical table with its current operational state.
type TableOverview struct {
	Table       Table
	CurrentGame *TableGameSession
	Clients     []Client
}

// History represents an action taken in the system, like a sale or game start.
type History struct {
	ID             int64
	Kind           string // 'sale', 'entry', 'game'
	ClientID       *int64
	ProductID      *int64
	GameID         *int64
	TableID        *int64
	VisitID        *int64
	Description    string
	Quantity       int
	UnitPriceCents int64
	TotalCents     int64
	CreatedAt      time.Time
}
