package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"leprechaun/internal/discord"
)

// Every env var the app reads. Listed here so missing config fails fast at
// startup with a clear message, instead of crashing later (DISCORD_KEY) or
// silently submitting blank fields to Google Forms (the G_FORM_* keys).
var requiredEnv = []string{
	"DISCORD_KEY",
	"G_FORM_LINK",
	"G_FORM_TITLE_ENTRY",
	"G_FORM_PRICE_ENTRY",
	"G_FORM_CATEGORY_ENTRY",
	"G_FORM_CATEGORY_D_VALUE",
	"G_FORM_TIMESTAMP_ENTRY",
}

func main() {
	// JSON to stderr so Portainer / docker logs / a log aggregator can parse fields.
	// Level defaults to INFO; override with LOG_LEVEL=debug|warn|error to retune
	// without a redeploy.
	level := slog.LevelInfo
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	// .env is optional: a missing file is fine when env vars are supplied by
	// the orchestrator (Portainer, docker run -e, k8s, ...). A malformed file
	// is still fatal — we don't want silent typos.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Error("loading .env", "err", err)
		os.Exit(1)
	}

	var missing []string
	for _, k := range requiredEnv {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		slog.Error("missing required env vars", "missing", missing)
		os.Exit(1)
	}

	discord.Token = os.Getenv("DISCORD_KEY")
	discord.Run()

	slog.Info("shutting down")
}
