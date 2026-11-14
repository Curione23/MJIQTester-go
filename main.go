package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	cfg     Config
	banlist *Banlist
	state   *VerificationState
	quiz    *QuizManager
}

func main() {
	configPath := "config.cfg"
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	banlist := NewBanlist(cfg.VerificationBanlistPath)
	if err := banlist.Load(); err != nil {
		log.Printf("warning: failed to load banlist: %v", err)
	}
	bot := &Bot{cfg: cfg, banlist: banlist, state: NewVerificationState(), quiz: NewQuizManager()}

	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %v", err)
	}
	dg.Identify.Intents = discordgo.IntentsGuilds

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as %s", r.User.Username)
	})
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		bot.handleInteraction(s, i)
	})

	if err := dg.Open(); err != nil {
		log.Fatalf("error opening connection: %v", err)
	}
	defer dg.Close()

	if err := registerCommands(dg, cfg.ClientID); err != nil {
		log.Printf("failed to register commands: %v", err)
	}

	fmt.Println("Bot is running. Press CTRL-C to exit.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("Shutting down.")
}

