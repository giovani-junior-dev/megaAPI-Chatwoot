package main

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/madeinlowcode/chatwoot-megaapi-bridge/internal/bridge"
)

func cmdAdmin(ctx context.Context, log zerolog.Logger, args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: bridge admin add --email <e> --password <p>")
	}
	return cmdAdminAdd(ctx, log, args[1:])
}

func cmdAdminAdd(ctx context.Context, log zerolog.Logger, args []string) error {
	email, password, err := parseAdminFlags(args)
	if err != nil {
		return err
	}
	hash, err := bridge.HashPassword(password)
	if err != nil {
		return err
	}
	dsn, err := loadDSN()
	if err != nil {
		return err
	}
	db, err := bridge.NewDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.UpsertAdmin(ctx, email, hash); err != nil {
		return err
	}
	log.Info().Str("email", email).Msg("admin upserted")
	fmt.Printf("Admin upserted: %s\n", email)
	return nil
}

func parseAdminFlags(args []string) (string, string, error) {
	fs := flag.NewFlagSet("admin add", flag.ContinueOnError)
	var email, password string
	fs.StringVar(&email, "email", "", "admin e-mail")
	fs.StringVar(&password, "password", "", "admin password")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if email == "" || password == "" {
		return "", "", errors.New("--email and --password are required")
	}
	return email, password, nil
}
