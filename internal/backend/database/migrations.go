package database

// migrations are applied in order; never edit one that has shipped, append a
// new one instead. Money is stored in cents (INTEGER) to avoid rounding.
var migrations = []string{
	// 1: initial schema.
	`
CREATE TABLE clients (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	name          TEXT    NOT NULL,
	document      TEXT,
	active_card   INTEGER CHECK (active_card IS NULL OR active_card BETWEEN 0 AND 999),
	checked_in_at TEXT,
	created_at    TEXT    NOT NULL,
	updated_at    TEXT    NOT NULL
);
CREATE UNIQUE INDEX ux_clients_active_card ON clients(active_card) WHERE active_card IS NOT NULL;
CREATE INDEX ix_clients_name ON clients(name COLLATE NOCASE);
CREATE INDEX ix_clients_document ON clients(document);

CREATE TABLE products (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	category    TEXT    NOT NULL DEFAULT 'Outros',
	price_cents INTEGER NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
	stock       INTEGER NOT NULL DEFAULT 0,
	min_stock   INTEGER NOT NULL DEFAULT 0,
	image       TEXT    NOT NULL DEFAULT '',
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);
CREATE INDEX ix_products_name ON products(name COLLATE NOCASE);

CREATE TABLE games (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	min_players INTEGER NOT NULL DEFAULT 1,
	max_players INTEGER NOT NULL DEFAULT 4,
	notes       TEXT    NOT NULL DEFAULT '',
	image       TEXT    NOT NULL DEFAULT '',
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT    NOT NULL,
	updated_at  TEXT    NOT NULL
);
CREATE INDEX ix_games_name ON games(name COLLATE NOCASE);

CREATE TABLE tables (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	game_id    INTEGER REFERENCES games(id) ON DELETE SET NULL,
	active     INTEGER NOT NULL DEFAULT 1,
	created_at TEXT    NOT NULL
);

-- Who sat at which table and when. Rows are never deleted: left_at closes them.
CREATE TABLE tables_clients (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	table_id  INTEGER NOT NULL REFERENCES tables(id),
	client_id INTEGER NOT NULL REFERENCES clients(id),
	joined_at TEXT    NOT NULL,
	left_at   TEXT
);
CREATE INDEX ix_tables_clients_open ON tables_clients(table_id) WHERE left_at IS NULL;
CREATE INDEX ix_tables_clients_client ON tables_clients(client_id);

-- Everything that happened: sales, entry fees and games played.
CREATE TABLE history (
	id               INTEGER PRIMARY KEY AUTOINCREMENT,
	kind             TEXT    NOT NULL CHECK (kind IN ('sale', 'entry', 'game')),
	client_id        INTEGER REFERENCES clients(id),
	product_id       INTEGER REFERENCES products(id),
	game_id          INTEGER REFERENCES games(id),
	table_id         INTEGER REFERENCES tables(id),
	description      TEXT    NOT NULL DEFAULT '',
	quantity         INTEGER NOT NULL DEFAULT 1,
	unit_price_cents INTEGER NOT NULL DEFAULT 0,
	total_cents      INTEGER NOT NULL DEFAULT 0,
	created_at       TEXT    NOT NULL
);
CREATE INDEX ix_history_created ON history(created_at);
CREATE INDEX ix_history_client ON history(client_id, created_at);

CREATE TABLE configuration (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`,
	// 2: customer visits, checkout snapshots, and table game sessions.
	`
CREATE TABLE visits (
	id                           INTEGER PRIMARY KEY AUTOINCREMENT,
	client_id                    INTEGER NOT NULL REFERENCES clients(id),
	client_name                  TEXT    NOT NULL,
	client_document              TEXT,
	card_number                  INTEGER NOT NULL CHECK (card_number BETWEEN 0 AND 999),
	entry_price_cents            INTEGER NOT NULL DEFAULT 0 CHECK (entry_price_cents >= 0),
	consumption_subtotal_cents   INTEGER NOT NULL DEFAULT 0 CHECK (consumption_subtotal_cents >= 0),
	gross_total_cents            INTEGER NOT NULL DEFAULT 0 CHECK (gross_total_cents >= 0),
	discount_cents               INTEGER NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
	final_total_cents            INTEGER NOT NULL DEFAULT 0 CHECK (final_total_cents >= 0),
	payment_method               TEXT    NOT NULL DEFAULT '',
	receipt_identifier           TEXT,
	opened_at                    TEXT    NOT NULL,
	closed_at                    TEXT,
	receipt_issued_at            TEXT
);
CREATE UNIQUE INDEX ux_visits_open_client ON visits(client_id) WHERE closed_at IS NULL;
CREATE UNIQUE INDEX ux_visits_receipt_identifier ON visits(receipt_identifier) WHERE receipt_identifier IS NOT NULL;
CREATE INDEX ix_visits_opened ON visits(opened_at);
CREATE INDEX ix_visits_closed ON visits(closed_at);

ALTER TABLE history ADD COLUMN visit_id INTEGER REFERENCES visits(id);
CREATE INDEX ix_history_visit ON history(visit_id, created_at);

CREATE TABLE table_game_sessions (
	id                   INTEGER PRIMARY KEY AUTOINCREMENT,
	table_id             INTEGER NOT NULL REFERENCES tables(id),
	game_id              INTEGER REFERENCES games(id),
	brought_game_name    TEXT    NOT NULL DEFAULT '',
	brought_by_client_id INTEGER REFERENCES clients(id),
	started_at           TEXT    NOT NULL,
	ended_at             TEXT,
	CHECK (
		(game_id IS NOT NULL AND brought_game_name = '') OR
		(game_id IS NULL AND length(trim(brought_game_name)) > 0)
	),
	CHECK (game_id IS NULL OR brought_by_client_id IS NULL)
);
CREATE UNIQUE INDEX ux_table_game_sessions_open ON table_game_sessions(table_id) WHERE ended_at IS NULL;
CREATE INDEX ix_table_game_sessions_table ON table_game_sessions(table_id, started_at);

INSERT INTO visits (
	client_id, client_name, client_document, card_number, entry_price_cents,
	opened_at
)
SELECT id, name, document, active_card, 0, COALESCE(NULLIF(checked_in_at, ''), created_at)
FROM clients
WHERE active_card IS NOT NULL;

INSERT INTO table_game_sessions (table_id, game_id, brought_game_name, started_at)
SELECT id, game_id, '', created_at
FROM tables
WHERE game_id IS NOT NULL;
`,
}
