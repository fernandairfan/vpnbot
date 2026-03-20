package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/yourgithub/fansstorevpn-go-final/internal/config"
	"github.com/yourgithub/fansstorevpn-go-final/internal/db"
	"github.com/yourgithub/fansstorevpn-go-final/internal/telegram"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatal(err)
	}
	store, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	bot := telegram.New(cfg, store)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Println("FANSSTOREVPN started")
	if err := bot.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
