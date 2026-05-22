# Leprechaun — context for Claude

Small personal-use Go (1.19) Discord bot. One purpose: turn a Silpo e-receipt URL pasted into a Discord channel into one row per item in a Google Form (which backs a Sheet used as a personal expense tracker).

Read `readme.md` for the user-facing summary; this file captures what's useful when editing the code.

## Data flow

```
Discord message ──► bot/bot.go (newMessage)
  contains "receipt.silpo"
        │
        ▼
processing/processor.go (ParseSilpoLink)
        │
        ├─► parsers/html.go (GetHtml)      ── HTTP GET with spoofed Chrome UA
        │
        ├─► parsers/silpo.go (ParseSilpoChequeHtml)
        │     walks the parsed DOM, extracts []ChequeItem + receipt time.Time
        │
        └─► submitting/google.go (SubmitGoogleForm)
              one http.PostForm per item, 100 ms apart
              returns a human summary string
        ▼
bot replies in the same channel
```

Everything runs in a goroutine spawned per receipt message — `bot/bot.go:60`. There is no queue, no cancellation, no shutdown drain.

## Entry points

- `main.go` — loads `.env`, sets `bot.Token`, calls `bot.Run()`. Hard-fails if `.env` is missing.
- `bot/bot.go:21` `Run()` — opens the Discord session, blocks on SIGINT.
- `bot/bot.go:42` `newMessage` — the command router. To add a command, add a `case` here.
- `processing/processor.go:9` `ParseSilpoLink(url)` — the only orchestration function; fetch → parse → submit.

## Parser conventions (parsers/silpo.go)

The Silpo HTML is non-semantic and printer-flavored. The parser is a manual recursive walk over `golang.org/x/net/html` nodes with a few assumptions worth knowing before touching it:

- Termination signal is `dateTime` being non-zero. Once the device-info `ЧАС` (time) cell is parsed, the walker short-circuits. Items must therefore appear **before** the device info in document order — they currently do.
- A row is recognized as a goods row when its first `<td>` has `class="... no-break ..."` and the text does **not** contain `уцінка` (discount).
- Two row shapes are handled:
  - Single `<td>` for title plus a price column → straight title + price.
  - Two-`<tr>` shape with a `2 X 55.49` quantity line → title gets the quantity string appended, price is read from the second row's price td.
- `class` checks use `n.Attr[0].Val` — they assume `class` is the *first* attribute. If Silpo reorders attributes, parsing silently produces empty results.
- Timestamp format from the page is `15:04:05 02.01.2006` (HH:MM:SS DD.MM.YYYY).
- Loop safety: `getSilpoChequeItems` bails after 200 tbody iterations to avoid infinite loops from unexpected markup.
- A reference receipt is pinned in a block comment at the bottom of the file — keep it in sync if the parser logic changes.

## Google Form submission (submitting/google.go)

- One `http.PostForm` per item, with a `100 * time.Millisecond` sleep between calls ("dont overwhelm google"). Don't remove the sleep without testing — Google has rate-limited bulk submissions in the past.
- Timestamp is sent as five fields (`_year`, `_month`, `_day`, `_hour`, `_minute`) appended to the base entry id. A commented-out single-field form is left for reference if the form is reconfigured.
- `ChequeItem.Category` is never set; the form gets a fixed `G_FORM_CATEGORY_D_VALUE`.
- Any non-200 from Google aborts the whole receipt; already-submitted items are not rolled back.

## Known sharp edges

These are listed for awareness — don't fix them as part of an unrelated change without asking.

- `bot/bot.go:15` `checkNilErr` discards the real error and `log.Fatal`s a literal `"Error message"`.
- `bot/bot.go:31` `discord.Open()` error is ignored.
- `parsers/html.go:13–14` uses `req` even if `http.NewRequest` returned an error (`err` is overwritten by `client.Do`).
- Hardcoded `User-Agent` is ancient (Chrome 39). Some sites block on UA; if Silpo starts 403'ing again (see commit `49f07cd`), rotate this.
- `parsers/silpo.go:185` checks `titleTd.FirstChild.Data` for `"уцінка"` — but `FirstChild` can be nil for empty cells. Hasn't crashed in practice; flag if you change row handling.
- No tests, no linter config, no CI.

## Conventions to follow when editing

- Go 1.19, stdlib + `discordgo` + `godotenv` + `golang.org/x/net/html`. Do not add dependencies for marginal wins.
- Errors are wrapped with `fmt.Errorf("context: %v", err)` (note: `%v`, not `%w` — no `errors.Is/As` use anywhere yet). Match the style unless you're introducing unwrapping intentionally.
- No structured logging — plain `log` and `fmt.Println`.
- Configuration is environment-only, read via `os.Getenv` at call sites (not loaded into a config struct).
- Commands are dispatched by `strings.Contains` on the message body, which is loose by design. New commands should pick a token unlikely to appear in normal chat (e.g. a `!prefix`).
- The personal-use nature means UX defaults to "fail loudly to the channel" rather than retry / queue.

## Things this bot is *not*

- Not multi-tenant: one Google Form, one category, one set of credentials in `.env`.
- Not a generic receipt parser: only Silpo's `receipt.silpo` HTML format.
- Not a long-running service: no health endpoint, no metrics, no retry, no persistence.
