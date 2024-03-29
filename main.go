package main

import (
	"github.com/joho/godotenv"
	"leprechaun/bot"
	"log"
	"os"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bot.Token = os.Getenv("DISCORD_KEY")
	bot.Run() // call the run function of bot/bot.go

	log.Println("Shutting down")
}
