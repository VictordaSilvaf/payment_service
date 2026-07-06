package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"payment_service/internal/database/migrate"
	"payment_service/internal/infrastructure/config"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg := config.Load()
	dsn := cfg.Postgres.DSN()

	var err error
	switch os.Args[1] {
	case "up":
		err = migrate.Up(dsn)
	case "down":
		err = migrate.Down(dsn)
	case "steps":
		if len(os.Args) < 3 {
			log.Fatal("usage: migrate steps <n>")
		}
		n, parseErr := strconv.Atoi(os.Args[2])
		if parseErr != nil {
			log.Fatal(parseErr)
		}
		err = migrate.Steps(dsn, n)
	case "version":
		version, dirty, verErr := migrate.Version(dsn)
		if verErr != nil {
			log.Fatal(verErr)
		}
		fmt.Printf("version: %d, dirty: %v\n", version, dirty)
		return
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {
	fmt.Println(`Usage:
  migrate up          Apply all pending migrations
  migrate down        Rollback all migrations
  migrate steps <n>   Apply n migrations (negative to rollback)
  migrate version     Show current migration version`)
}
