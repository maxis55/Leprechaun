// Package discord is the Discord transport. It opens the bot session, routes
// messages, and hands recognized receipt URLs to the receipt package. All
// retailer-specific logic lives in receipt — discord/ only knows hosts.
package discord

import (
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"leprechaun/internal/receipt"
)

// Token is the Discord bot token, set by main before calling Run.
var Token string

// First http(s) URL found in a message.
var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

type retailer struct {
	name       string
	hostSuffix string
	parse      receipt.Parser
}

// Each retailer is matched by a host suffix so receipt.silpo.* (currently
// receipt.silpo.elkasa.com.ua) keeps working if the e-receipt hostname changes
// behind the same prefix. Add new retailers here and add a matching parser in
// the receipt package.
var retailers = []retailer{
	{"silpo", "receipt.silpo", receipt.ParseSilpo},
	{"varus", "ecom-gateway.varus.ua", receipt.ParseVarus},
}

func retailerFor(host string) (retailer, bool) {
	for _, r := range retailers {
		if strings.HasPrefix(host, r.hostSuffix) || strings.HasSuffix(host, r.hostSuffix) {
			return r, true
		}
	}
	return retailer{}, false
}

// Run opens the Discord session and blocks until SIGINT or SIGTERM.
func Run() {
	session, err := discordgo.New("Bot " + Token)
	if err != nil {
		slog.Error("discordgo.New failed", "err", err)
		os.Exit(1)
	}

	session.AddHandler(onMessage)

	if err := session.Open(); err != nil {
		// Without this check, an invalid/revoked token produces a healthy-looking
		// container that logs "bot running" and silently does nothing.
		slog.Error("opening discord session failed", "err", err)
		os.Exit(1)
	}
	defer session.Close()

	slog.Info("bot running")

	// SIGINT (Ctrl+C, local) or SIGTERM (docker stop / Portainer stop button).
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func onMessage(session *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == session.State.User.ID {
		return
	}

	switch {
	case strings.Contains(m.Content, "!help"):
		session.ChannelMessageSend(m.ChannelID, "Hello World😃")
		return
	case strings.Contains(m.Content, "!bye"):
		session.ChannelMessageSend(m.ChannelID, "Good Bye👋")
		return
	}

	link := urlRe.FindString(m.Content)
	if link == "" {
		return
	}
	u, err := url.Parse(link)
	if err != nil {
		return
	}
	r, ok := retailerFor(u.Host)
	if !ok {
		return
	}

	session.ChannelMessageSend(m.ChannelID, "Parsing")
	go handleReceipt(session, m.ChannelID, r, link)
}

func handleReceipt(session *discordgo.Session, channelID string, r retailer, link string) {
	log := slog.With("retailer", r.name, "url", link, "channel", channelID)
	log.Info("processing receipt")

	res, err := receipt.Process(link, r.parse)
	if err != nil {
		log.Error("receipt failed", "err", err)
		session.ChannelMessageSend(channelID,
			"Couldn't process this receipt. Open the link in a browser to confirm it loads; check the bot logs for the detailed error.")
		return
	}

	log.Info("receipt submitted", "summary", strings.TrimSpace(res))
	session.ChannelMessageSend(channelID, res)
}
