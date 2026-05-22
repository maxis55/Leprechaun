# Leprechaun — context for Claude

Small personal-use Go (1.25) Discord bot. One purpose: turn a Ukrainian grocery e-receipt URL (Silpo or Varus) pasted into a Discord channel into one row per item in a Google Form (which backs a Sheet used as a personal expense tracker).

Read `readme.md` for the user-facing summary; this file captures what's useful when editing the code.

## Data flow

```
Discord message ──► bot/bot.go (newMessage)
  matches "receipt.silpo"  OR  "ecom-gateway.varus.ua"
        │
        ▼
processing/processor.go (ParseSilpoLink | ParseVarusLink)
        │
        ├─► parsers/html.go (GetHtml)              ── HTTP GET with spoofed Chrome UA
        │
        ├─► parsers/{silpo,varus}.go               ── goquery-based DOM extraction
        │     returns []ChequeItem + receipt time.Time
        │
        └─► submitting/google.go (SubmitGoogleForm)
              one http.PostForm per item, 100 ms apart
              returns a human summary string
        ▼
bot replies in the same channel
```

Everything runs in a goroutine spawned per receipt message — `bot/bot.go` `newMessage`. There is no queue, no cancellation, no shutdown drain. Each goroutine swallows its own error into a Discord reply.

## Entry points

- `main.go` — loads `.env`, sets `bot.Token`, calls `bot.Run()`. Hard-fails if `.env` is missing.
- `bot/bot.go` `Run()` — opens the Discord session, blocks on SIGINT.
- `bot/bot.go` `newMessage` — the command router. To add a new retailer, add a `case` here and a `ParseXxxLink` in processing.
- `processing/processor.go` — one orchestration function per retailer (`ParseSilpoLink`, `ParseVarusLink`). Pattern: `GetHtml → ParseXxxChequeHtml → SubmitGoogleForm`.

## Parser conventions

Both parsers are written with `github.com/PuerkitoBio/goquery` (jQuery-style CSS selectors over `golang.org/x/net/html`). They share `ChequeItem` (defined in `parsers/silpo.go`) and `parsePrice` (in `parsers/html.go`).

Output convention: weighed and multi-unit quantities are **appended to the title** in the same string field, separated by a space. There is no quantity column on the form. `Submit` only reads `Title` and `Price`. Examples:
- `"Балик ІНДИЧИЙ к/в Саяйвір ваг 0.192"` (Varus weighed)
- `"Нап500КакаоGelЩенПат 2 X 129.00"` (Silpo multi-unit)
- `"Хл330ЦарХлТостРанЦіл"` (Silpo single unit — no qty suffix)

Discounts are intentionally dropped: the gross (pre-discount) price is recorded. This matches the project's existing policy (commit `f82c566` "Ignore discount").

### Silpo (`parsers/silpo.go`)

- Items live under `table.cheque-goods > tbody`, one item per `<tbody>`.
- The title row is identified as **the first `<tr>` whose first `<td>` has class `no-break`** — *not* a fixed index. Alcohol and other excise items prepend extra `<tr>`s containing a UKT-ZED code and an internal product code; the `no-break` scan skips them. A code comment in `parseSilpoItems` flags this — do not replace the scan with positional indexing or alcohol parsing will silently break.
- Two title-row shapes:
  - **Inline shape** (most items): title `<td>` + price `<td>` in the same `<tr>`.
  - **Stacked shape** (weighed/multi-unit, e.g. `БананКг`): title `<tr>` with only the title cell, followed by a `<tr>` whose first cell is `0.768 X 73.90` (or `2 X 129.00`) and the second is the line total. The qty cell text is appended to the title.
- Rows whose title contains `"уцінка"` (markdown/clearance) are skipped — they represent a discount on a previous line, not a separate purchase.
- Timestamp: `.device-info-line-item` cell containing `"ЧАС"`, value in the immediate next sibling, format `"15:04:05 02.01.2006"`.
- A historical reference cheque used to live in a block comment in this file. It was stale (used `<div>` for the device-info block where current Silpo uses `<td>`) and was removed in the goquery migration; saved fixtures under `.scratch/` (gitignored) are the new source of truth.

### Varus (`parsers/varus.go`)

- Items live under `tr.service`.
- Title cell contains, in order: a barcode `<p>` (wraps a `<span>` — skipped by checking `p.Children().Length() > 0`), the title `<p>`, and optionally a `Знижка` `<p>` on discounted items (skipped by exact text match).
- Quantity (`К-сть`) is the first non-empty `<p class="itemtext">` in the middle `<td>`. Appended to the title unless it's exactly `"1.000"` (suppress the noisy suffix for single-unit items).
- Price cell: first `<p class="itemtext">` is the gross line total (e.g. `"79.90   А"` — trailing tax letter stripped via regex `\d+\.\d+`). Second `<p>` (if present) is the discount amount and is ignored.
- Timestamp: `p.fscl-info-bot` whose text matches `^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}$`, format `"02.01.2006 15:04"` (no seconds — unlike Silpo).

## Google Form submission (submitting/google.go)

- One `http.PostForm` per item, with a `100 * time.Millisecond` sleep between calls ("dont overwhelm google"). Don't remove the sleep without testing — Google has rate-limited bulk submissions in the past.
- Timestamp is sent as five fields (`_year`, `_month`, `_day`, `_hour`, `_minute`) appended to the base entry id. A commented-out single-field form is left for reference if the form is reconfigured.
- `ChequeItem.Category` is never set; the form gets a fixed `G_FORM_CATEGORY_D_VALUE`.
- Any non-200 from Google aborts the whole receipt; already-submitted items are not rolled back.

## Known sharp edges

These are listed for awareness — don't fix them as part of an unrelated change without asking.

- `bot/bot.go` `checkNilErr` discards the real error and `log.Fatal`s a literal `"Error message"`.
- `bot/bot.go` `discord.Open()` error is ignored.
- `parsers/html.go` `GetHtml` uses `req` even if `http.NewRequest` returned an error (`err` is overwritten by `client.Do`).
- Hardcoded `User-Agent` is ancient (Chrome 39). Some sites block on UA; if Silpo starts 403'ing again (see commit `49f07cd`), rotate this.
- No tests, no linter config, no CI. The `.scratch/` directory is the de-facto fixture store for manual verification.

## Conventions to follow when editing

- Go 1.25, pinned in both `go.mod` and `Dockerfile` (`golang:1.25-alpine`). Direct deps: `discordgo`, `godotenv`, `goquery`, `golang.org/x/net`. Do not add dependencies for marginal wins.
- Errors are wrapped with `fmt.Errorf("context: %v", err)` (note: `%v`, not `%w` — no `errors.Is/As` use anywhere yet). Match the style unless you're introducing unwrapping intentionally.
- No structured logging — plain `log` and `fmt.Println`.
- Configuration is environment-only, read via `os.Getenv` at call sites (not loaded into a config struct).
- Commands are dispatched by `strings.Contains` on the message body, which is loose by design. New commands should pick a token unlikely to appear in normal chat (e.g. a host string or `!prefix`).
- The personal-use nature means UX defaults to "fail loudly to the channel" rather than retry / queue.

## Things this bot is *not*

- Not multi-tenant: one Google Form, one category, one set of credentials in `.env`.
- Not a generic receipt parser: only the specific HTML formats of supported retailers (currently Silpo + Varus).
- Not a long-running service: no health endpoint, no metrics, no retry, no persistence.
