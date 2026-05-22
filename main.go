package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"leprechaun/bot"
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
	// .env is optional: a missing file is fine when env vars are supplied by
	// the orchestrator (Portainer, docker run -e, k8s, ...). A malformed file
	// is still a fatal — we don't want silent typos.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatalf("loading .env: %v", err)
	}

	var missing []string
	for _, k := range requiredEnv {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		log.Fatalf("missing required env vars: %v", missing)
	}

	bot.Token = os.Getenv("DISCORD_KEY")
	bot.Run()

	log.Println("Shutting down")
}
