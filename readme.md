# Leprechaun

Discord bot that parses Ukrainian grocery e-receipts (Silpo and Varus) and forwards each line item to a Google Form, one row per item.

## What it does

1. Listens to all messages in channels the bot can see.
2. When a message contains a recognized receipt host, it fetches the HTML, parses out each line item (title + price) and the receipt timestamp.
   - `receipt.silpo` → Silpo parser
   - `ecom-gateway.varus.ua` → Varus parser
3. POSTs one Google Form submission per item, with a fixed category and the receipt timestamp split into year/month/day/hour/minute.
4. Replies in the channel with the number of items submitted, total cost, and timestamp.

For weighed and multi-unit items, the quantity (`0.768 X 73.90`, `1.024`, etc.) is appended to the item title in the same form field — there is no separate quantity field.

Discounts (`Знижка` on Varus, `ЗНИЖКА` line on Silpo) are intentionally ignored: items are recorded at gross price.

It also answers `!help` (`Hello World😃`) and `!bye` (`Good Bye👋`).

## Configure

Copy `.env.example` to `.env` and fill in:

| Variable | Purpose |
| --- | --- |
| `DISCORD_KEY` | Discord bot token. |
| `G_FORM_LINK` | The Google Form `formResponse` POST URL. |
| `G_FORM_TITLE_ENTRY` | Form field id (e.g. `entry.123456`) for the item title. |
| `G_FORM_PRICE_ENTRY` | Form field id for the item price. |
| `G_FORM_CATEGORY_ENTRY` | Form field id for the category. |
| `G_FORM_CATEGORY_D_VALUE` | The single category value sent for every item (the bot does not categorize). |
| `G_FORM_TIMESTAMP_ENTRY` | Base field id for the timestamp; the bot appends `_year`, `_month`, `_day`, `_hour`, `_minute`. |

## Run

Local:

```sh
go run main.go
```

Docker:

```sh
docker build -t leprechaun .
docker run --env-file .env leprechaun
```

The bot stays up until `Ctrl+C`.

## Use from Discord

Paste a recognized receipt URL into a channel the bot is in:

- Silpo: anything containing `receipt.silpo` (e.g. `https://receipt.silpo.elkasa.com.ua/dMICd9BWWR4`)
- Varus: anything containing `ecom-gateway.varus.ua` (e.g. `https://ecom-gateway.varus.ua/public/api/e-receipt/view/<uuid>`)

It replies `Parsing`, then either an error or a summary like:

```
Submitted 7 items, total cost is 1684.00, timestamp is 31/12/2023 14:41:35
```

## Layout

```
main.go              loads .env, starts the bot
bot/bot.go           Discord session + message router
processing/          orchestrates fetch -> parse -> submit (one func per chain)
parsers/html.go      generic HTTP GET with a browser User-Agent + shared parsePrice
parsers/silpo.go     Silpo cheque parser (goquery selectors)
parsers/varus.go     Varus cheque parser (goquery selectors)
submitting/google.go fires one POST per item to the Google Form
```

## Known limitations

- `checkNilErr` calls `log.Fatal("Error message")` and throws away the real error. Real errors from `discordgo.New` will be hard to diagnose.
- `discord.Open()`'s error is ignored.
- `parsers.GetHtml` checks `err` from `client.Do` but uses the request even if `http.NewRequest` failed; also sends an ancient Chrome 39 User-Agent that has triggered 403s in the past.
- Category is a single hardcoded value (`G_FORM_CATEGORY_D_VALUE`); the `Category` field on `ChequeItem` is never populated.
- The parsers are tied to the current HTML markup of each retailer — there is no schema or contract. Site redesigns will break them.
- No tests.
- An in-flight receipt parse is not drained on shutdown; `Ctrl+C` can interrupt mid-submit.
- Each form submission sleeps 100 ms; a 50-item receipt takes ~5 s.
