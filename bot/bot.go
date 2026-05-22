package bot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"leprechaun/processing"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var Token string

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message")
	}
}

func Run() {

	// create a session
	discord, err := discordgo.New("Bot " + Token)
	checkNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// Block until interrupt (Ctrl+C, local) or SIGTERM (docker/portainer stop).
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {

	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// respond to user message if it contains `!help` or `!bye`
	switch {
	case strings.Contains(message.Content, "!help"):
		discord.ChannelMessageSend(message.ChannelID, "Hello World😃")
	case strings.Contains(message.Content, "!bye"):
		discord.ChannelMessageSend(message.ChannelID, "Good Bye👋")
	case strings.Contains(message.Content, "receipt.silpo"):
		discord.ChannelMessageSend(message.ChannelID, "Parsing")
		go func() {
			res, err := processing.ParseSilpoLink(strings.Replace(message.Content, "!parseSilpo ", "", 1))
			if err != nil {
				discord.ChannelMessageSend(message.ChannelID, err.Error())
			} else {
				discord.ChannelMessageSend(message.ChannelID, res)
			}
		}()
	case strings.Contains(message.Content, "ecom-gateway.varus.ua"):
		discord.ChannelMessageSend(message.ChannelID, "Parsing")
		go func() {
			res, err := processing.ParseVarusLink(strings.Replace(message.Content, "!parseVarus ", "", 1))
			if err != nil {
				discord.ChannelMessageSend(message.ChannelID, err.Error())
			} else {
				discord.ChannelMessageSend(message.ChannelID, res)
			}
		}()
	}

}
