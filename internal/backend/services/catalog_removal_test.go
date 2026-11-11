package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

func TestProductRemovalPoliciesAndValidation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	service := NewProductService(db)

	unused := models.Product{Name: "  Água  ", Category: "  Bebidas  ", PriceCents: 500, Stock: 4, MinStock: 1, Active: true}
	if err := service.Create(ctx, &unused); err != nil {
		t.Fatal(err)
	}
	if unused.ID == 0 || unused.Name != "Água" || unused.Category != "Bebidas" {
		t.Fatalf("created product was not normalized: %+v", unused)
	}
	plan, err := service.PlanRemoval(ctx, unused.ID)
	if err != nil || plan.Kind != models.RemovalDelete {
		t.Fatalf("unused product plan = %+v, %v", plan, err)
	}
	removed, err := service.Remove(ctx, unused.ID)
	if err != nil || removed.Kind != models.RemovalDelete {
		t.Fatalf("unused product removal = %+v, %v", removed, err)
	}
	assertRowCount(t, db, "products", unused.ID, 0)

	referenced := models.Product{Name: "Café", Category: "Bebidas", PriceCents: 900, Stock: 5, MinStock: 1, Active: true}
	if err := service.Create(ctx, &referenced); err != nil {
		t.Fatal(err)
	}
	createdID := referenced.ID
	now := time.Now().Format(database.TimeLayout)
	if _, err := db.ExecContext(ctx, `INSERT INTO history(kind, product_id, description, created_at) VALUES('sale', ?, 'Café vendido', ?)`, referenced.ID, now); err != nil {
		t.Fatal(err)
	}
	referenced.Name = "Café especial"
	if err := service.Update(ctx, &referenced); err != nil {
		t.Fatal(err)
	}
	if referenced.ID != createdID {
		t.Fatalf("product update changed ID from %d to %d", createdID, referenced.ID)
	}
	var snapshot string
	if err := db.QueryRowContext(ctx, "SELECT description FROM history WHERE product_id = ?", referenced.ID).Scan(&snapshot); err != nil || snapshot != "Café vendido" {
		t.Fatalf("sale snapshot = %q, %v", snapshot, err)
	}
	removed, err = service.Remove(ctx, referenced.ID)
	if err != nil || removed.Kind != models.RemovalArchive {
		t.Fatalf("referenced product removal = %+v, %v", removed, err)
	}
	if containsProductID(t, service, ctx, true, referenced.ID) || !containsProductID(t, service, ctx, false, referenced.ID) {
		t.Fatal("archived product visibility is incorrect")
	}
	referenced.Active = true
	if err := service.Update(ctx, &referenced); err != nil || !containsProductID(t, service, ctx, true, referenced.ID) {
		t.Fatalf("explicit product reactivation failed: %v", err)
	}

	originalName, originalPrice := referenced.Name, referenced.PriceCents
	for _, invalid := range []models.Product{
		{ID: referenced.ID, Name: "   ", Category: "Bebidas", PriceCents: originalPrice, Active: true},
		{ID: referenced.ID, Name: originalName, Category: "Bebidas", PriceCents: -1, Active: true},
		{ID: referenced.ID, Name: originalName, Category: "Bebidas", Stock: -1, Active: true},
		{ID: referenced.ID, Name: originalName, Category: "Bebidas", MinStock: -1, Active: true},
	} {
		if err := service.Update(ctx, &invalid); err == nil {
			t.Fatalf("invalid product update succeeded: %+v", invalid)
		}
	}
	var storedName string
	var storedPrice int64
	if err := db.QueryRowContext(ctx, "SELECT name, price_cents FROM products WHERE id = ?", referenced.ID).Scan(&storedName, &storedPrice); err != nil {
		t.Fatal(err)
	}
	if storedName != originalName || storedPrice != originalPrice {
		t.Fatalf("invalid update mutated product to %q/%d", storedName, storedPrice)
	}
	invalidCreate := models.Product{Name: "Novo", Category: "Bebidas", PriceCents: -1, Active: true}
	if err := service.Create(ctx, &invalidCreate); err == nil || invalidCreate.ID != 0 {
		t.Fatalf("invalid create = id %d, err %v", invalidCreate.ID, err)
	}
}

func TestGameRemovalPoliciesAndValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("unused game is deleted", func(t *testing.T) {
		db := testDB(t)
		service := NewGameService(db)
		game := models.Game{Name: " Azul ", MinPlayers: 2, MaxPlayers: 4, Active: true}
		if err := service.Create(ctx, &game); err != nil {
			t.Fatal(err)
		}
		if game.Name != "Azul" || game.ID == 0 {
			t.Fatalf("created game = %+v", game)
		}
		plan, err := service.Remove(ctx, game.ID)
		if err != nil || plan.Kind != models.RemovalDelete {
			t.Fatalf("unused game removal = %+v, %v", plan, err)
		}
		assertRowCount(t, db, "games", game.ID, 0)
	})

	for _, reference := range []string{"history", "session", "table"} {
		t.Run("referenced by "+reference+" is archived", func(t *testing.T) {
			db := testDB(t)
			service := NewGameService(db)
			game := models.Game{Name: "Catan", MinPlayers: 3, MaxPlayers: 4, Active: true}
			if err := service.Create(ctx, &game); err != nil {
				t.Fatal(err)
			}
			tableID := createTableForRemoval(t, db, "Mesa 1")
			now := time.Now().Format(database.TimeLayout)
			switch reference {
			case "history":
				_, err := db.ExecContext(ctx, `INSERT INTO history(kind, game_id, description, created_at) VALUES('game', ?, 'Catan', ?)`, game.ID, now)
				if err != nil {
					t.Fatal(err)
				}
			case "session":
				_, err := db.ExecContext(ctx, `INSERT INTO table_game_sessions(table_id, game_id, started_at, ended_at) VALUES(?, ?, ?, ?)`, tableID, game.ID, now, now)
				if err != nil {
					t.Fatal(err)
				}
			case "table":
				if _, err := db.ExecContext(ctx, "UPDATE tables SET game_id = ? WHERE id = ?", game.ID, tableID); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := service.PlanRemoval(ctx, game.ID)
			if err != nil || plan.Kind != models.RemovalArchive {
				t.Fatalf("referenced game plan = %+v, %v", plan, err)
			}
			removed, err := service.Remove(ctx, game.ID)
			if err != nil || removed.Kind != models.RemovalArchive {
				t.Fatalf("referenced game removal = %+v, %v", removed, err)
			}
			var active int
			if err := db.QueryRowContext(ctx, "SELECT active FROM games WHERE id = ?", game.ID).Scan(&active); err != nil || active != 0 {
				t.Fatalf("archived game active=%d, err=%v", active, err)
			}
		})
	}

	db := testDB(t)
	service := NewGameService(db)
	invalidCreate := models.Game{Name: " ", MinPlayers: 1, MaxPlayers: 4, Active: true}
	if err := service.Create(ctx, &invalidCreate); err == nil || invalidCreate.ID != 0 {
		t.Fatalf("invalid game create = id %d, err %v", invalidCreate.ID, err)
	}
	game := models.Game{Name: "Dixit", MinPlayers: 3, MaxPlayers: 6, Active: true}
	if err := service.Create(ctx, &game); err != nil {
		t.Fatal(err)
	}
	originalID := game.ID
	game.Name = "Dixit Odyssey"
	if err := service.Update(ctx, &game); err != nil || game.ID != originalID {
		t.Fatalf("valid game update id=%d err=%v", game.ID, err)
	}
	for _, invalid := range []models.Game{
		{ID: game.ID, Name: " ", MinPlayers: 1, MaxPlayers: 4, Active: true},
		{ID: game.ID, Name: "Dixit", MinPlayers: 0, MaxPlayers: 4, Active: true},
		{ID: game.ID, Name: "Dixit", MinPlayers: 4, MaxPlayers: 3, Active: true},
	} {
		if err := service.Update(ctx, &invalid); err == nil {
			t.Fatalf("invalid game update succeeded: %+v", invalid)
		}
	}
	var storedName string
	if err := db.QueryRowContext(ctx, "SELECT name FROM games WHERE id = ?", game.ID).Scan(&storedName); err != nil || storedName != "Dixit Odyssey" {
		t.Fatalf("invalid update changed game to %q, %v", storedName, err)
	}
}

func TestGameRemovalArchivedGameCannotStartSession(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	game := models.Game{Name: "Catan", MinPlayers: 3, MaxPlayers: 4, Active: true}
	if err := NewGameService(db).Create(ctx, &game); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(database.TimeLayout)
	if _, err := db.ExecContext(ctx, `INSERT INTO history(kind, game_id, description, created_at) VALUES('game', ?, 'Catan', ?)`, game.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGameService(db).Remove(ctx, game.ID); err != nil {
		t.Fatal(err)
	}
	tableID := createTableForRemoval(t, db, "Mesa 1")
	generateAccessCodes(t, db, 1)
	visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "001")
	if err != nil {
		t.Fatal(err)
	}
	if err := NewTablesClientsService(db).AssignClient(ctx, tableID, visit.ClientID); err != nil {
		t.Fatal(err)
	}
	if err := NewTableGameService(db).StartCatalog(ctx, tableID, game.ID); err == nil {
		t.Fatal("archived game started a new session")
	}
}

func TestTableRemovalPoliciesRaceAndRelationships(t *testing.T) {
	ctx := context.Background()

	t.Run("unused table is deleted", func(t *testing.T) {
		db := testDB(t)
		service := NewTableService(db)
		table := models.Table{Name: " Mesa 1 ", Active: true}
		if err := service.Create(ctx, &table); err != nil {
			t.Fatal(err)
		}
		if table.Name != "Mesa 1" || table.ID == 0 {
			t.Fatalf("created table = %+v", table)
		}
		plan, err := service.Remove(ctx, table.ID)
		if err != nil || plan.Kind != models.RemovalDelete {
			t.Fatalf("unused table removal = %+v, %v", plan, err)
		}
		assertRowCount(t, db, "tables", table.ID, 0)
	})

	t.Run("historical table is archived and plan is rechecked", func(t *testing.T) {
		db := testDB(t)
		service := NewTableService(db)
		table := models.Table{Name: "Mesa 2", Active: true}
		if err := service.Create(ctx, &table); err != nil {
			t.Fatal(err)
		}
		before, err := service.PlanRemoval(ctx, table.ID)
		if err != nil || before.Kind != models.RemovalDelete {
			t.Fatalf("initial table plan = %+v, %v", before, err)
		}
		now := time.Now().Format(database.TimeLayout)
		if _, err := db.ExecContext(ctx, `INSERT INTO history(kind, table_id, description, created_at) VALUES('game', ?, 'Mesa usada', ?)`, table.ID, now); err != nil {
			t.Fatal(err)
		}
		after, err := service.Remove(ctx, table.ID)
		if err != nil || after.Kind != models.RemovalArchive {
			t.Fatalf("raced table removal = %+v, %v", after, err)
		}
		var active int
		if err := db.QueryRowContext(ctx, "SELECT active FROM tables WHERE id = ?", table.ID).Scan(&active); err != nil || active != 0 {
			t.Fatalf("archived table active=%d, err=%v", active, err)
		}
	})

	t.Run("occupied or playing table is blocked", func(t *testing.T) {
		db := testDB(t)
		service := NewTableService(db)
		occupied := models.Table{Name: "Ocupada", Active: true}
		if err := service.Create(ctx, &occupied); err != nil {
			t.Fatal(err)
		}
		generateAccessCodes(t, db, 1)
		visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "001")
		if err != nil {
			t.Fatal(err)
		}
		if err := NewTablesClientsService(db).AssignClient(ctx, occupied.ID, visit.ClientID); err != nil {
			t.Fatal(err)
		}
		plan, err := service.PlanRemoval(ctx, occupied.ID)
		if err != nil || plan.Kind != models.RemovalBlocked {
			t.Fatalf("occupied table plan = %+v, %v", plan, err)
		}
		if removed, err := service.Remove(ctx, occupied.ID); err == nil || removed.Kind != models.RemovalBlocked {
			t.Fatalf("occupied table removal = %+v, %v", removed, err)
		}

		playing := models.Table{Name: "Em jogo", Active: true}
		if err := service.Create(ctx, &playing); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Format(database.TimeLayout)
		if _, err := db.ExecContext(ctx, `INSERT INTO table_game_sessions(table_id, brought_game_name, started_at) VALUES(?, 'Azul', ?)`, playing.ID, now); err != nil {
			t.Fatal(err)
		}
		plan, err = service.PlanRemoval(ctx, playing.ID)
		if err != nil || plan.Kind != models.RemovalBlocked {
			t.Fatalf("playing table plan = %+v, %v", plan, err)
		}
	})

	t.Run("rename preserves seating and session IDs", func(t *testing.T) {
		db := testDB(t)
		service := NewTableService(db)
		table := models.Table{Name: "Mesa antiga", Active: true}
		if err := service.Create(ctx, &table); err != nil {
			t.Fatal(err)
		}
		generateAccessCodes(t, db, 1)
		visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "001")
		if err != nil {
			t.Fatal(err)
		}
		if err := NewTablesClientsService(db).AssignClient(ctx, table.ID, visit.ClientID); err != nil {
			t.Fatal(err)
		}
		if err := NewTableGameService(db).StartBrought(ctx, table.ID, "Azul", nil); err != nil {
			t.Fatal(err)
		}
		originalID := table.ID
		table.Name = "Mesa nova"
		if err := service.Update(ctx, &table); err != nil || table.ID != originalID {
			t.Fatalf("table update id=%d err=%v", table.ID, err)
		}
		var seating, sessions int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tables_clients WHERE table_id = ?", table.ID).Scan(&seating); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM table_game_sessions WHERE table_id = ?", table.ID).Scan(&sessions); err != nil {
			t.Fatal(err)
		}
		if seating != 1 || sessions != 1 {
			t.Fatalf("relationships after rename: seating=%d sessions=%d", seating, sessions)
		}
		invalidCreate := models.Table{Name: " ", Active: true}
		if err := service.Create(ctx, &invalidCreate); err == nil || invalidCreate.ID != 0 {
			t.Fatalf("invalid table create = id %d, err %v", invalidCreate.ID, err)
		}
		invalid := models.Table{ID: table.ID, Name: " ", Active: true}
		if err := service.Update(ctx, &invalid); err == nil {
			t.Fatal("blank table name was accepted")
		}
	})
}

func assertRowCount(t *testing.T, db *sql.DB, table string, id int64, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE id = ?", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s row count for %d = %d, want %d", table, id, count, want)
	}
}

func containsProductID(t *testing.T, service *ProductService, ctx context.Context, onlyActive bool, id int64) bool {
	t.Helper()
	products, err := service.List(ctx, onlyActive)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range products {
		if product.ID == id {
			return true
		}
	}
	return false
}

func createTableForRemoval(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	table := models.Table{Name: name, Active: true}
	if err := NewTableService(db).Create(context.Background(), &table); err != nil {
		t.Fatal(err)
	}
	return table.ID
}
