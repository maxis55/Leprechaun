# Leprechaun

Discord bot that parses Silpo (Ukrainian grocery chain) e-receipts and forwards each line item to a Google Form, one row per item.

## What it does

1. Listens to all messages in channels the bot can see.
2. When a message contains `receipt.silpo` (a public Silpo e-receipt URL), it fetches the HTML, parses out each line item (title + price) and the receipt timestamp.
3. POSTs one Google Form submission per item, with a fixed category and the receipt timestamp split into year/month/day/hour/minute.
4. Replies in the channel with the number of items submitted, total cost, and timestamp.

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

Paste a Silpo e-receipt URL (anything containing `receipt.silpo`) into a channel the bot is in. It replies `Parsing`, then either an error or a summary like:

```
Submitted 7 items, total cost is 1684.00, timestamp is 31/12/2023 14:41:35
```

## Layout

```
main.go              loads .env, starts the bot
bot/bot.go           Discord session + message router
processing/          orchestrates fetch -> parse -> submit
parsers/html.go      generic HTTP GET with a browser User-Agent
parsers/silpo.go     Silpo cheque HTML walker (golang.org/x/net/html)
submitting/google.go fires one POST per item to the Google Form
```

## Known limitations

- `checkNilErr` calls `log.Fatal("Error message")` and throws away the real error. Real errors from `discordgo.New` will be hard to diagnose.
- `discord.Open()`'s error is ignored.
- `parsers.GetHtml` checks `err` from `client.Do` but uses the request even if `http.NewRequest` failed.
- Category is a single hardcoded value (`G_FORM_CATEGORY_D_VALUE`); the `Category` field on `ChequeItem` is never populated.
- The Silpo parser is brittle: it relies on DOM order, `class` attribute being the first attribute, and skips rows marked `уцінка` (discount). Site markup changes will break it.
- No tests.
- An in-flight receipt parse is not drained on shutdown; `Ctrl+C` can interrupt mid-submit.
- Each form submission sleeps 100 ms; a 50-item receipt takes ~5 s.
