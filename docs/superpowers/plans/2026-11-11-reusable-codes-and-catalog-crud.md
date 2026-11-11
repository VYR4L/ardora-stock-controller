# Reusable Codes and Catalog CRUD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reusable, printable QR/Code 128 command codes with contextual scanning, safe edit/remove flows for tables, products, and games, and fix the empty-command layout overflow.

**Architecture:** Add explicit `access_code_batches` and `access_codes` persistence, make an access code the source of truth for an open visit, and keep `card_number` as a compatibility snapshot. A dedicated code service owns normalization, generation, lookup, availability, and printing; the sales UI consumes those interfaces through focused scanner fields. Catalog services expose transactional removal plans so the UI can distinguish permanent deletion, archival, and blocked operations.

**Tech Stack:** Go 1.27.1, MyGo v0.3.7, SQLite via `modernc.org/sqlite`, `github.com/boombuler/barcode` for pure-Go QR and Code 128 generation, HTML/CSS A4 printing.

**Spec:** `docs/superpowers/specs/2026-11-11-reusable-codes-and-catalog-crud-design.md`

## Global Constraints

- Runtime behavior remains local and offline; no network is required to generate, read, or print codes.
- Published migrations are immutable; append migration 3 instead of editing migrations 1 or 2.
- The first release accepts only numeric command codes in the inclusive `000–999` range; generated batches start at `001` unless a larger legacy number already exists.
- QR Code and Code 128 on one label carry the exact same three-digit, zero-padded payload.
- Scanning at check-in validates and fills the code only; `Registrar entrada` is the sole action that creates the visit and assigns the code.
- Product scanning is contextual: a product and positive quantity must be selected before `Enter` on the command-code field can create one charge.
- Checkout scanning opens the existing preview and never closes or charges automatically.
- A code is available when it is active and has no open visit; checkout releases it without deleting its historical visit relationships.
- Permanent removal is allowed only for never-referenced products, games, and tables; otherwise use `active = false`, and block table archival while occupied or in an active game.
- Existing financial snapshots remain immutable; catalog/table/game names read through live relationships reflect corrected names.
- Supported visual verification sizes remain `1024×768` and `800×600`, in light and dark themes.

## Review Focus

- Scanner input may include whitespace, a one-digit manual value, or two rapid `Enter` events; normalize to one canonical value and never create two concurrent charges from one in-flight submission (Tasks 2 and 7).
- A legacy code may occur in many closed visits and one open visit; migration must create one code, link every visit, and report it in use only while the open visit exists (Task 1).
- Batch generation at the upper bound must create `999` when it fits and roll back the entire request when any requested number would exceed `999` (Task 2).
- A record can become referenced or a table can become occupied between confirmation and removal; the transactional recheck must archive or block according to the new state instead of trusting stale UI data (Task 8).
- Reprinting and OS print failures must never advance the sequence, mutate code payloads, or remove the saved HTML path (Task 4).

---

### Task 1: Persist access-code batches and migrate legacy visits

**Files:**
- Modify: `internal/backend/database/migrations.go`
- Modify: `internal/backend/database/migrations_test.go`
- Modify: `internal/backend/models/models.go`

**Interfaces:**
- Produces: `models.AccessCodeBatch`, `models.AccessCode`, `models.AccessCodeState`, `models.AccessCodeStats`.
- Produces: nullable `models.Visit.AccessCodeID` and canonical `models.Visit.AccessCodeValue`.
- Produces: migration 3 with `access_code_batches`, `access_codes`, `visits.access_code_id`, foreign keys, lookup indexes, and a partial unique index on open visits by access code.

- [ ] **Step 1: Write failing fresh-database and upgrade tests**

Add `TestFreshDatabaseAppliesAccessCodeSchema` and `TestUpgradeBackfillsLegacyAccessCodes` with assertions equivalent to:

```go
if gotTables != []string{"access_code_batches", "access_codes"} { t.Fatalf(...) }
if code.Value != "007" || code.State != models.AccessCodeInUse { t.Fatalf(...) }
if closedVisitCodeID != openVisitCodeID { t.Fatal("legacy visits did not share one code") }
if _, err := db.Exec(`INSERT INTO visits (..., access_code_id, closed_at) VALUES (..., ?, NULL)`, openVisitCodeID); err == nil {
    t.Fatal("expected unique open-code constraint")
}
```

Seed both a closed visit and an open visit with card `7`, plus a closed-only card `8`; assert code `007` is in use and `008` is available after migration.

- [ ] **Step 2: Run the migration tests and verify they fail**

Run: `go test ./internal/backend/database -run 'Test(FreshDatabaseAppliesAccessCodeSchema|UpgradeBackfillsLegacyAccessCodes)' -v`

Expected: FAIL because migration 3 and the access-code model types do not exist.

- [ ] **Step 3: Add access-code model types**

Add these exact domain fields in `internal/backend/models/models.go`:

```go
type AccessCodeState string
const (
    AccessCodeAvailable AccessCodeState = "available"
    AccessCodeInUse     AccessCodeState = "in_use"
    AccessCodeArchived  AccessCodeState = "archived"
)
type AccessCodeBatch struct {
    ID int64; FirstNumber, LastNumber, Quantity int; Legacy bool
    CreatedAt time.Time; LastPrintedAt *time.Time
}
type AccessCode struct {
    ID, BatchID int64; Number int; Value string; Active bool
    State AccessCodeState; CreatedAt time.Time; ArchivedAt *time.Time
}
type AccessCodeStats struct { Total, Available, InUse, Archived, NextNumber int }
```

Extend `models.Visit` with `AccessCodeID *int64` and `AccessCodeValue string`.

- [ ] **Step 4: Append migration 3**

Create immutable batches with `legacy INTEGER NOT NULL DEFAULT 0`, codes with unique `number` and `value`, and `active/archived_at`. Backfill one legacy batch only when historical card numbers exist, insert one code per distinct number with `printf('%03d', card_number)`, link all visits, and create `ux_visits_open_access_code` with `WHERE closed_at IS NULL AND access_code_id IS NOT NULL`.

- [ ] **Step 5: Run the database tests**

Run: `go test ./internal/backend/database -v`

Expected: PASS, including fresh installs, v1/v2 upgrades, shared legacy codes, and the open-visit uniqueness constraint.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/database/migrations.go internal/backend/database/migrations_test.go internal/backend/models/models.go
git commit -m "feat: persist reusable command codes"
```

### Task 2: Implement normalization, generation, listing, and code lifecycle

**Files:**
- Create: `internal/backend/services/access_codes.go`
- Create: `internal/backend/services/access_codes_test.go`

**Interfaces:**
- Consumes: access-code tables and model types from Task 1.
- Produces: `NormalizeAccessCode(raw string) (string, error)`.
- Produces: `NewAccessCodeService(db *sql.DB) *AccessCodeService`.
- Produces: `GenerateBatch(ctx context.Context, quantity int) (*models.AccessCodeBatch, error)`.
- Produces: `ListBatches(ctx context.Context) ([]models.AccessCodeBatch, error)` and `ListBatchCodes(ctx context.Context, batchID int64) ([]models.AccessCode, error)`.
- Produces: `ListCodes(ctx context.Context, query string) ([]models.AccessCode, error)` and `Stats(ctx context.Context) (models.AccessCodeStats, error)`.
- Produces: `Resolve(ctx context.Context, raw string) (*models.AccessCode, error)`, `ResolveAvailable(ctx context.Context, raw string) (*models.AccessCode, error)`, `Archive(ctx context.Context, codeID int64) error`, and `Reactivate(ctx context.Context, codeID int64) error`.

- [ ] **Step 1: Write failing normalization and generation tests**

Add table tests asserting:

```go
NormalizeAccessCode(" 7\r\n") == "007"
NormalizeAccessCode("000") == "000"
NormalizeAccessCode("-1"), NormalizeAccessCode("1000"), NormalizeAccessCode("ABC") return errors
GenerateBatch(ctx, 2) creates 001, 002 and the next batch starts at 003
GenerateBatch(ctx, 0) returns an error and inserts no rows
```

Seed max code `998`; assert generating one creates `999`, while generating two returns an error and changes neither batches nor codes.

Start two goroutines generating non-overlapping quantities against the same database and assert both batches commit with contiguous, unique ranges; the single SQLite writer may serialize them, but neither caller may observe or create a duplicated number.

- [ ] **Step 2: Run focused tests and verify they fail**

Run: `go test ./internal/backend/services -run 'TestAccessCode' -v`

Expected: FAIL because `AccessCodeService` and `NormalizeAccessCode` are undefined.

- [ ] **Step 3: Implement canonical normalization and transactional batch generation**

Use trimmed ASCII digits only, parse to `0–999`, and return `fmt.Sprintf("%03d", number)`. In `GenerateBatch`, begin one transaction, choose `max(number)+1` with a minimum of `1`, validate the full requested interval before inserting, then insert the batch and every code.

- [ ] **Step 4: Write failing state/list/archive tests**

Assert that one open visit makes its code `in_use`, checkout makes it `available`, archived codes disappear from `ResolveAvailable`, archiving an in-use code fails, reactivation restores availability, and a query matches both `007` and its numeric form `7`.

- [ ] **Step 5: Implement list, statistics, resolution, archive, and reactivation methods**

Derive state with an `EXISTS` check against open visits rather than storing a second availability flag. `Archive` must recheck for an open visit inside its transaction before setting `active = 0, archived_at = now`.

- [ ] **Step 6: Run service tests**

Run: `go test ./internal/backend/services -run 'TestAccessCode' -v`

Expected: PASS, including canonicalization, boundary rollback, state derivation, and lifecycle constraints.

- [ ] **Step 7: Commit**

```bash
git add internal/backend/services/access_codes.go internal/backend/services/access_codes_test.go
git commit -m "feat: manage reusable command code batches"
```

### Task 3: Make access codes the visit and billing source of truth

**Files:**
- Modify: `internal/backend/services/visits.go`
- Modify: `internal/backend/services/visits_test.go`
- Modify: `internal/backend/services/sales.go`
- Modify: `internal/backend/models/models.go`
- Modify: `internal/backend/services/table_games_test.go`
- Modify: `internal/backend/services/backup_test.go`

**Interfaces:**
- Consumes: `NormalizeAccessCode` and access-code persistence from Tasks 1–2.
- Changes: `VisitService.OpenVisit(ctx context.Context, name string, document *string, codeValue string) (*models.Visit, error)`.
- Changes: `SalesService.RegisterClient(ctx context.Context, name string, document *string, codeValue string) (*models.Client, error)`.
- Produces: `VisitService.ActiveForCode(ctx context.Context, raw string) (*models.Visit, error)`.
- Produces: `SalesService.BillProductByCode(ctx context.Context, raw string, productID int64, qty int) (*models.Visit, error)`.
- Preserves: `SalesService.BillProduct(ctx, clientID, productID int64, qty int) error` for existing internal callers/tests.

- [ ] **Step 1: Rewrite visit tests against generated codes and add transactional assignment cases**

Generate codes before opening visits and change calls from integer cards to canonical strings. Add assertions:

```go
visit, err := visits.OpenVisit(ctx, "Ana", nil, " 1 ")
if visit.AccessCodeValue != "001" || visit.AccessCodeID == nil { t.Fatalf(...) }
if _, err := visits.OpenVisit(ctx, "Bia", nil, "001"); err == nil { t.Fatal(...) }
if got, err := visits.ActiveForCode(ctx, "1"); err != nil || got.ID != visit.ID { t.Fatalf(...) }
```

Add a trigger-forced insert failure and assert the code remains available because client update, visit insert, entry history, and code assignment roll back together.

- [ ] **Step 2: Run visit and sales tests to verify the API change fails**

Run: `go test ./internal/backend/services -run 'Test(Visit|Billing|Checkout|AccessCode)' -v`

Expected: FAIL until `OpenVisit` and visit scanning use access codes.

- [ ] **Step 3: Update visit queries and transactional open/checkout behavior**

Join `access_codes` in `visitSelect`, scan `access_code_id/value`, normalize and lock the requested active code during `OpenVisit`, and store both `access_code_id` and its numeric snapshot. Keep checkout idempotent; closing a visit releases the code solely by changing `closed_at` while still clearing `clients.active_card` for compatibility.

- [ ] **Step 4: Add failing contextual billing tests**

Add `TestBillProductByCodeChargesExactlyOneOpenVisit` and assert the returned visit matches the code, stock decrements once, one history row is created, an available-without-visit code fails, and an archived/unknown code fails without changing stock.

- [ ] **Step 5: Implement `ActiveForCode` and `BillProductByCode`**

Resolve the canonical code and active visit inside the billing transaction, then reuse one private `billProductTx(ctx, tx, visitID, clientID, productID, qty)` implementation for both billing entry points.

- [ ] **Step 6: Update existing fixtures and run all backend service tests**

Update table-game and backup tests to generate/assign access codes instead of assuming arbitrary integer cards exist.

Run: `go test ./internal/backend/services -v`

Expected: PASS with financial snapshots, table sessions, CSV backups, access-code release, and legacy behavior preserved.

- [ ] **Step 7: Commit**

```bash
git add internal/backend/models/models.go internal/backend/services/visits.go internal/backend/services/visits_test.go internal/backend/services/sales.go internal/backend/services/table_games_test.go internal/backend/services/backup_test.go
git commit -m "feat: assign command codes to visits"
```

### Task 4: Generate and reprint A4 QR plus Code 128 label sheets

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/backend/paths/paths.go`
- Create: `internal/backend/services/access_code_print.go`
- Create: `internal/backend/services/access_code_print_test.go`

**Interfaces:**
- Consumes: `AccessCodeService.ListBatchCodes` from Task 2 and `openLocalFile` from `receipts.go`.
- Produces: `paths.Layout.CodeSheets string`, rooted at `<Ardora>/code-sheets` and created by `Layout.Ensure()`.
- Produces: `NewAccessCodePrintService(db *sql.DB, sheetsDir string) *AccessCodePrintService`.
- Produces: `SaveBatch(ctx context.Context, batchID int64, autoPrint bool) (string, error)` and `OpenBatch(ctx context.Context, batchID int64, autoPrint bool) (string, error)`.

- [ ] **Step 1: Add the pinned pure-Go barcode dependency**

Run: `go get github.com/boombuler/barcode@v1.0.2`

Expected: `go.mod` and `go.sum` include `github.com/boombuler/barcode v1.0.2` without CGO dependencies.

- [ ] **Step 2: Write failing payload and HTML tests**

Add tests that create a two-code batch, call `SaveBatch`, and assert:

```go
qrBarcode.Content() == "001"
code128Barcode.Content() == "001"
strings.Count(html, `class="label"`) == 2
strings.Contains(html, "@page{size:A4")
strings.Contains(html, "grid-template-columns:repeat(3")
strings.Contains(html, "001") && strings.Contains(html, "002")
```

Call `SaveBatch` twice and assert identical payload labels, unchanged batch/code counts, and unchanged next number. Stub `opener` to fail and assert `OpenBatch` returns both the saved path and an error.

- [ ] **Step 3: Run print tests and verify they fail**

Run: `go test ./internal/backend/services -run 'TestAccessCodePrint' -v`

Expected: FAIL because the print service and code-sheet path do not exist.

- [ ] **Step 4: Implement sheet rendering**

Generate QR with `qr.Encode(value, qr.M, qr.Auto)` and Code 128 with `code128.Encode(value)`, scale them to readable PNG data URIs, and render escaped HTML. Use A4 portrait, 10 mm margins, and a fixed 3-column × 4-row grid (12 labels per page); each label shows `ARDORA`, the large number, QR, Code 128, and the visible value.

- [ ] **Step 5: Save and open sheets without sequence mutation**

Save deterministic batch filenames such as `codigos-lote-000001.html`. Update `last_printed_at` only after the HTML is written successfully; opening failure must return the path for retry and must not alter codes or sequence.

- [ ] **Step 6: Run print and path tests**

Run: `go test ./internal/backend/services ./internal/backend/paths -run 'Test(AccessCodePrint|.*Layout.*)' -v`

Expected: PASS with stable payloads, A4 pagination CSS, repeatable output, and retryable open failures.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/backend/paths/paths.go internal/backend/services/access_code_print.go internal/backend/services/access_code_print_test.go
git commit -m "feat: print qr and barcode command labels"
```

### Task 5: Add the Codes navigation area and batch operations

**Files:**
- Modify: `internal/frontend/icons/icons.go`
- Modify: `internal/frontend/state.go`
- Create: `internal/frontend/tabs/codes.go`
- Create: `internal/frontend/tabs/codes_test.go`
- Modify: `internal/frontend/tabs/render_test.go`
- Modify: `main.go`
- Modify: `main_test.go`

**Interfaces:**
- Consumes: `AccessCodeService` from Task 2 and `AccessCodePrintService` from Task 4.
- Produces: `frontend.State.AccessCodeService` and `frontend.State.AccessCodePrintService`.
- Produces: `NewCodesTab(state *frontend.State) *CodesTab`, `(*CodesTab).Refresh()`, and `(*CodesTab).View(c *ui.Context)`.
- Produces: shell destination key `Codes` with visible label `Códigos`.

- [ ] **Step 1: Write failing tab helper and render tests**

Add tests for quantity validation (`1–999` and remaining capacity), search/state labels, batch option-to-ID mapping, and representative states: empty, generated batch, archived code, generation error, and print error. Extend shell render tests to select `Codes` at both supported sizes.

- [ ] **Step 2: Run the frontend tests and verify they fail**

Run: `go test ./internal/frontend/tabs . -run 'Test(Codes|RepresentativeTabStates|RenderAppShell)' -v`

Expected: FAIL because the tab, state services, icon, and route do not exist.

- [ ] **Step 3: Wire code services and navigation**

Construct `AccessCodeService` and `AccessCodePrintService` in `App.init`, add them to `frontend.State`, add a simple QR/barcode-style `icons.Codes`, create `codesTab`, refresh it on navigation, and route `Codes` in `viewSelectedTab`.

- [ ] **Step 4: Implement the Codes tab**

Show metrics for total/available/in-use/archived, the next code, generation quantity, batch history with `Imprimir`, and a searchable code list. Generate asynchronously, refresh only after success, print through `OpenBatch`, and offer archive/reactivate actions only when the current state allows them.

- [ ] **Step 5: Run tab and shell tests**

Run: `go test ./internal/frontend/tabs . -run 'Test(Codes|RepresentativeTabStates|RenderAppShell)' -v`

Expected: PASS at `1024×768` and `800×600`, light and dark.

- [ ] **Step 6: Commit**

```bash
git add internal/frontend/icons/icons.go internal/frontend/state.go internal/frontend/tabs/codes.go internal/frontend/tabs/codes_test.go internal/frontend/tabs/render_test.go main.go main_test.go
git commit -m "feat: add command code management screen"
```

### Task 6: Fix panel sizing and the empty-command overflow

**Files:**
- Modify: `internal/frontend/components/components.go`
- Modify: `internal/frontend/tabs/sales.go`
- Modify: `internal/frontend/tabs/tables.go`
- Modify: `internal/frontend/tabs/settings.go`
- Modify: `internal/frontend/tabs/render_test.go`

**Interfaces:**
- Changes: `components.Panel` remains `func Panel(...) ui.Element` but no longer applies `.Grow(1)` internally.
- Requires: callers that are direct children of horizontal rows explicitly append `.Grow(1)`.

- [ ] **Step 1: Add a failing layout regression test**

Add a `sales_empty_with_receipt` render state with no active visits and one completed receipt. Capture the returned panel bounds during a headless render and assert:

```go
if openPanel.Y+openPanel.H > receiptsPanel.Y {
    t.Fatalf("open-command panel overlaps receipts: open=%v receipts=%v", openPanel, receiptsPanel)
}
```

Run this state at `1024×768` and `800×600` in both themes and keep preview output behind `ARDORA_PREVIEW_DIR`.

- [ ] **Step 2: Run the regression test and verify it fails**

Run: `go test ./internal/frontend/tabs -run 'TestSalesEmptyStateDoesNotOverlapReceipts' -v`

Expected: FAIL with overlapping vertical bounds under the current unconditional panel growth.

- [ ] **Step 3: Make panel growth contextual**

Remove `.Grow(1)` from `components.Panel`. Add `.Grow(1)` to the two sales form panels, the two table form panels, and the three settings panels that are direct children of wrapping rows. Leave vertically stacked panels content-sized.

- [ ] **Step 4: Run component/tab render tests**

Run: `go test ./internal/frontend/tabs ./internal/frontend/components -v`

Expected: PASS, with separated empty/open and receipts panels and preserved horizontal sharing.

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/components/components.go internal/frontend/tabs/sales.go internal/frontend/tabs/tables.go internal/frontend/tabs/settings.go internal/frontend/tabs/render_test.go
git commit -m "fix: prevent empty command panel overflow"
```

### Task 7: Add contextual scanner flows to check-in, billing, and checkout

**Files:**
- Modify: `internal/frontend/tabs/sales.go`
- Modify: `internal/frontend/tabs/tables.go`
- Create: `internal/frontend/tabs/sales_scanner_test.go`
- Modify: `internal/frontend/tabs/render_test.go`

**Interfaces:**
- Consumes: `AccessCodeService.ResolveAvailable`, `VisitService.OpenVisit`, `VisitService.ActiveForCode`, and `SalesService.BillProductByCode`.
- Produces: `(*SalesTab).validateRegistrationCode()`, `(*SalesTab).billScannedCode()`, and `(*SalesTab).locateCheckoutCode()` as the `OnSubmit` handlers for three focused text inputs.
- Produces: a `salesActions` test seam whose function fields exactly mirror those four consumed service methods, plus `update func(func())`; `NewSalesTab` binds real services/window updates and tests bind synchronous fakes.

- [ ] **Step 1: Write failing scanner-state tests**

Use MyGo `ui.NewTester` with `SalesTab.actions` and `SalesTab.update` bound to synchronous fakes to assert:

```go
check-in Enter canonicalizes "7" to "007" and shows "Comanda 007 disponível" without opening a visit
Registrar entrada opens exactly one visit and clears the code after success
product Enter without product/positive quantity does not call billing
two Enter events while billBusy is true result in one billing call
checkout Enter opens preview for the matching visit and does not call Checkout
```

- [ ] **Step 2: Run scanner tests and verify they fail**

Run: `go test ./internal/frontend/tabs -run 'TestSalesScanner' -v`

Expected: FAIL because the contextual fields and handlers are absent.

- [ ] **Step 3: Replace integer-card check-in input with code validation**

Rename copy to **Código da comanda**, attach `.OnSubmit(t.validateRegistrationCode)`, display the canonical validated number, and keep `Registrar entrada` explicit. On button click, call `OpenVisit` with the raw/current code so the backend transaction revalidates availability; do not persist anything from the `OnSubmit` handler.

Load active/available codes with the sales refresh, add a **Selecionar código disponível** fallback that writes the same input value, and send selection, typing, and scanner input through the same validation handler.

- [ ] **Step 4: Implement product-then-scan billing**

Keep product and quantity controls, replace the active-visit selector with a scanner/manual command-code field, and attach `.OnSubmit(t.billScannedCode)`. Set `billBusy` before starting the goroutine, call `BillProductByCode`, clear only the code field after success, retain product selection for fast repeated service, and refresh stock/visits.

- [ ] **Step 5: Implement checkout lookup by code**

Add **Localizar comanda por código** above the open-command list. On submit, call `ActiveForCode`; on success call the existing `openCheckout(visit.ID)`. Preserve per-row **Fechar comanda** buttons as a mouse fallback.

- [ ] **Step 6: Update copy, focus order, and render states**

Use “comanda” consistently instead of “cartão” in sales and table labels/options, add scanner feedback/error states, and update keyboard focus tests to cover name → document → check-in code and product → quantity → billing code.

- [ ] **Step 7: Run sales tests**

Run: `go test ./internal/frontend/tabs -run 'Test(Sales|RepresentativeTabStates)' -v`

Expected: PASS, including explicit check-in confirmation, one in-flight charge, and non-destructive checkout lookup.

- [ ] **Step 8: Commit**

```bash
git add internal/frontend/tabs/sales.go internal/frontend/tabs/tables.go internal/frontend/tabs/sales_scanner_test.go internal/frontend/tabs/render_test.go
git commit -m "feat: add contextual command code scanning"
```

### Task 8: Implement transactional safe removal for products, games, and tables

**Files:**
- Modify: `internal/backend/models/models.go`
- Modify: `internal/backend/services/products.go`
- Modify: `internal/backend/services/games.go`
- Modify: `internal/backend/services/tables.go`
- Create: `internal/backend/services/catalog_removal_test.go`
- Modify: `internal/backend/services/backup_test.go`
- Modify: `internal/backend/services/table_games_test.go`

**Interfaces:**
- Produces: `models.RemovalKind` values `delete`, `archive`, `blocked` and `models.RemovalPlan{Kind RemovalKind, Message string}`.
- Produces on each of `ProductService`, `GameService`, and `TableService`: `PlanRemoval(ctx context.Context, id int64) (models.RemovalPlan, error)` and `Remove(ctx context.Context, id int64) (models.RemovalPlan, error)`.
- Preserves: existing `Create`, `List`, and `Update`; `Update` becomes the reactivation path through `Active = true`.

- [ ] **Step 1: Write failing product removal tests**

Assert an unused product is deleted, a product referenced by `history` is archived, archived products remain in `List(false)` but not `List(true)`, and a rename does not change the `history.description` financial snapshot.

For both create and update, assert the stored ID remains stable and invalid negative/blank values are rejected without mutating the existing row.

- [ ] **Step 2: Write failing game removal tests**

Assert an unused game is deleted; a game referenced by `history`, `table_game_sessions`, or `tables.game_id` is archived; archived games cannot start new sessions; and renaming changes live-joined history/CSV/session names without rewriting IDs.

Assert an update preserves the game ID and rejects an empty name or `max_players < min_players` without changing the stored row.

- [ ] **Step 3: Write failing table removal and race tests**

Assert an unused table is deleted, a referenced idle table is archived, and a currently occupied or actively-playing table returns `blocked`. For the race case, call `PlanRemoval` while unused, insert a reference, then call `Remove` and assert it archives rather than trusting the old delete plan.

Assert a table rename preserves its ID and existing seating/session relationships.

- [ ] **Step 4: Run removal tests and verify they fail**

Run: `go test ./internal/backend/services -run 'Test(Product|Game|Table)Removal' -v`

Expected: FAIL because removal plans and operations are not implemented.

- [ ] **Step 5: Implement shared removal model and product/game rules**

Inside each `Remove`, start a transaction, recompute its plan, then either `DELETE`, `UPDATE ... active = 0`, or return the blocked error. Use reference counts specific to each service and require one affected row.

- [ ] **Step 6: Implement table blocking and archive rules**

Check open `tables_clients` rows and open `table_game_sessions` before historical reference counts. Never auto-end seating or games during removal. An inactive record may be reactivated only through an explicit `Update` with `Active = true`.

- [ ] **Step 7: Add backend create/update validation**

Trim names/categories and reject empty product/game/table names, negative product price/stock/minimum, and game ranges where `min < 1` or `max < min` in both `Create` and `Update`; add focused assertions to `catalog_removal_test.go`.

- [ ] **Step 8: Run all backend tests**

Run: `go test ./internal/backend/... -v`

Expected: PASS, including transactional race rechecks and historical naming behavior.

- [ ] **Step 9: Commit**

```bash
git add internal/backend/models/models.go internal/backend/services/products.go internal/backend/services/games.go internal/backend/services/tables.go internal/backend/services/catalog_removal_test.go internal/backend/services/backup_test.go internal/backend/services/table_games_test.go
git commit -m "feat: add safe catalog removal policies"
```

### Task 9: Expose edit, delete, archive, and reactivate actions in catalog screens

**Files:**
- Create: `internal/frontend/components/confirmation.go`
- Modify: `internal/frontend/tabs/stock.go`
- Modify: `internal/frontend/tabs/games.go`
- Modify: `internal/frontend/tabs/tables.go`
- Modify: `internal/frontend/tabs/render_test.go`
- Modify: `internal/frontend/tabs/catalog_helpers_test.go`

**Interfaces:**
- Consumes: `PlanRemoval`, `Remove`, and existing `Update` methods from Task 8.
- Produces: `components.ConfirmationModal(c, open *bool, title, detail, confirmLabel string, danger bool, busy bool, confirm, cancel func())`.
- Produces: create/edit modes in `StockTab` and `GamesTab`, plus edit/remove modal state in `TablesTab`.

- [ ] **Step 1: Add failing edit-mode and confirmation-copy tests**

Assert opening edit preserves ID and stored image, saving calls `Update` rather than `Create`, and the confirmation label is exactly **Excluir definitivamente** for `delete` and **Arquivar** for `archive`. Assert `blocked` shows the service message without opening a destructive confirmation.

- [ ] **Step 2: Run frontend catalog tests and verify they fail**

Run: `go test ./internal/frontend/tabs -run 'Test(Product|Game|Table|Catalog).*' -v`

Expected: FAIL because cards expose no edit/remove actions.

- [ ] **Step 3: Add product edit/removal UI**

Add **Editar** and **Excluir** actions per product. Reuse the form with title **Novo produto** or **Editar produto**, preserve the existing image unless a replacement is chosen, expose an **Ativo** checkbox for reactivation, and call `Create` or `Update` by `formItem.ID == 0`. Resolve the plan before confirmation and refresh after removal.

- [ ] **Step 4: Add game edit/removal UI**

Mirror the product behavior for name, player range, notes, cover, and active state. Keep the existing image during edits and show the correct permanent/archive confirmation copy.

- [ ] **Step 5: Add table edit/removal UI**

Add **Editar mesa** and **Excluir** to each table card. Edit name and active state through `TableService.Update`; use `PlanRemoval`/`Remove` for deletion or archival and surface the occupied/active-game block message without changing seating or game sessions.

- [ ] **Step 6: Add render states for all modal variants**

Cover product/game/table edit, permanent-delete confirmation, archive confirmation, blocked feedback, and inactive/reactivation cards at both supported sizes and themes.

- [ ] **Step 7: Run frontend tests**

Run: `go test ./internal/frontend/... -v`

Expected: PASS with accessible labels, correct form modes, and unambiguous destructive copy.

- [ ] **Step 8: Commit**

```bash
git add internal/frontend/components/confirmation.go internal/frontend/tabs/stock.go internal/frontend/tabs/games.go internal/frontend/tabs/tables.go internal/frontend/tabs/render_test.go internal/frontend/tabs/catalog_helpers_test.go
git commit -m "feat: add catalog edit and safe remove actions"
```

### Task 10: Run end-to-end migration, rendering, and release verification

**Files:**
- Modify if needed: `README.md`
- Modify if needed: `docs/superpowers/specs/2026-11-11-reusable-codes-and-catalog-crud-design.md`
- Modify if needed: test files touched by Tasks 1–9

**Interfaces:**
- Consumes: every interface from Tasks 1–9.
- Produces: a verified local release with documented scanner/printing behavior and no pending test regressions.

- [ ] **Step 1: Format all changed Go files**

Run: `gofmt -w main.go main_test.go internal/backend internal/frontend`

Expected: command exits 0 and `git diff --check` reports no whitespace errors.

- [ ] **Step 2: Run the full automated suite**

Run: `go test ./...`

Expected: PASS for database migration, services, scanner behavior, CRUD policies, receipts, label sheets, and light/dark rendering.

- [ ] **Step 3: Generate visual regression previews**

Run in PowerShell:

```powershell
$env:ARDORA_PREVIEW_DIR = Join-Path $PWD 'tmp-previews'
go test ./internal/frontend/tabs . -run 'Test(RepresentativeTabStatesRenderAtSupportedSizes|RenderAppShellAtSupportedSizes|SalesEmptyStateDoesNotOverlapReceipts)' -count=1
```

Expected: preview PNGs exist for sales empty-with-receipt, scanner states, codes, and CRUD modals at `1024×768` and `800×600` in both themes; inspect them for clipping, overlapping, and unreadable destructive actions.

- [ ] **Step 4: Verify static analysis and build**

Run: `go vet ./...`

Expected: exit 0 with no diagnostics.

Run: `go build ./...`

Expected: exit 0.

- [ ] **Step 5: Validate OpenSpec artifacts if present**

Run: `npx --yes @fission-ai/openspec validate --all --strict`

Expected: exit 0; if this repository version requires a change name, validate the active Ardora change explicitly and record the exact command in the task notes.

- [ ] **Step 6: Update operator documentation**

Document that USB readers behave as keyboards ending in `Enter`, check-in still requires **Registrar entrada**, QR and Code 128 contain the same value, reprinting does not allocate codes, and catalog **Excluir** may become **Arquivar** when history exists.

- [ ] **Step 7: Commit final verification/docs changes**

```bash
git add README.md docs/superpowers/specs/2026-11-11-reusable-codes-and-catalog-crud-design.md
git diff --cached --quiet || git commit -m "docs: explain reusable command code workflow"
```
