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

The bot loads `.env` if present (best-effort) and then validates that every required variable is set and non-empty. If anything is missing it exits with code 1 and logs the list of missing keys — so it's safe to run with `.env`, with `--env-file`, with `-e` flags, or with env vars set by an orchestrator like Portainer. It will never run silently with a broken config.

Local:

```sh
go run main.go
```

Docker (local build, env via file):

```sh
docker build -t leprechaun .
docker run --env-file .env --restart unless-stopped leprechaun
```

Docker (env via flags, no `.env` file needed):

```sh
docker run \
  -e DISCORD_KEY=... \
  -e G_FORM_LINK=... \
  -e G_FORM_TITLE_ENTRY=... \
  -e G_FORM_PRICE_ENTRY=... \
  -e G_FORM_CATEGORY_ENTRY=... \
  -e G_FORM_CATEGORY_D_VALUE=... \
  -e G_FORM_TIMESTAMP_ENTRY=... \
  --restart unless-stopped \
  leprechaun
```

Docker Compose (uses the included `docker-compose.yml`, builds locally, reads env from `.env`):

```sh
docker compose --env-file .env up -d --build
```

The Dockerfile is a multi-stage build (`golang:1.25-alpine` builder → `alpine:3` runtime with `ca-certificates`). The final image is ~25 MB and runs as a non-root `app` user. A `.dockerignore` keeps `.env`, `.scratch/`, `.git/`, `.idea/`, and the Markdown docs out of the build context.

The bot stays up until `Ctrl+C` (SIGINT) or `docker stop` / Portainer's stop button (SIGTERM).

### Deploy in Portainer

Two options. Both work without pushing the image to a registry.

#### Option A — Build on the host, run via Portainer

Clone and build once on the Portainer host:

```sh
git clone https://github.com/<you>/leprechaun.git /opt/leprechaun
cd /opt/leprechaun
docker build -t leprechaun:local .
```

Then in Portainer → *Containers → Add container*:

- **Image**: `leprechaun:local` (Portainer reads it from the local Docker image store; leave "Always pull" off)
- **Env**: add each required variable
- **Restart policy**: `Unless stopped`

**Update workflow**:

```sh
cd /opt/leprechaun && git pull && docker build -t leprechaun:local .
```

Then in Portainer → the container → *Recreate* (with *Re-pull image* **off**). Or skip Portainer for the restart: `docker restart leprechaun`. Downtime is a couple of seconds.

#### Option B — Portainer clones + builds from git

Repo includes a `docker-compose.yml` with `build: .`, so Portainer can do the whole thing from the UI.

In Portainer → *Stacks → Add stack*:

1. **Build method**: *Repository*
2. **Repository URL**: `https://github.com/<you>/leprechaun.git`
3. **Compose path**: `docker-compose.yml`
4. **Environment variables**: add each required key
5. (optional) **Automatic updates**: poll the repo every N minutes; Portainer rebuilds and recreates when the branch advances.

**Update workflow** (manual): push to the branch, then in Portainer → the stack → *Pull and redeploy*. Portainer re-clones, rebuilds, and recreates the container. Or just turn on automatic updates and let it run.

#### Both options

If any required env var is missing or empty, the container exits 1 within a second of starting — check Portainer's container logs for a `missing required env vars: [...]` line.

**Secrets caveat.** Portainer stores env vars in its own database and shows them in the UI to anyone with access. That's fine for a personal bot; for stronger isolation use Docker secrets (would require code changes to read from `/run/secrets/...`).

No volumes or inbound ports are needed — the bot is stateless and only makes outbound HTTPS calls to Discord, the receipt hosts, and Google Forms.

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

- `checkNilErr` (in `bot/bot.go`) calls `log.Fatal("Error message")` and throws away the real error. Real errors from `discordgo.New` will be hard to diagnose.
- `discord.Open()`'s error is ignored, so an invalid `DISCORD_KEY` causes a silent no-op rather than a crash.
- `parsers.GetHtml` checks `err` from `client.Do` but uses the request even if `http.NewRequest` failed; also sends an ancient Chrome 39 User-Agent that has triggered 403s in the past.
- Category is a single hardcoded value (`G_FORM_CATEGORY_D_VALUE`); the `Category` field on `ChequeItem` is never populated.
- The parsers are tied to the current HTML markup of each retailer — there is no schema or contract. Site redesigns will break them.
- No tests.
- An in-flight receipt parse is not drained on shutdown; `Ctrl+C` can interrupt mid-submit.
- Each form submission sleeps 100 ms; a 50-item receipt takes ~5 s.
