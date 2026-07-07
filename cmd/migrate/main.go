package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"payment_service/internal/database/migrate"
	"payment_service/internal/infrastructure/config"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// "create" não precisa de conexão com o banco: apenas gera arquivos.
	if os.Args[1] == "create" {
		runCreate()
		return
	}

	cfg := config.Load()
	dsn := cfg.Postgres.DSN()

	switch os.Args[1] {
	case "up":
		fatalIf(migrate.Up(dsn))
	case "down":
		fatalIf(migrate.Down(dsn))
	case "steps":
		runSteps(dsn)
	case "version":
		runVersion(dsn)
	default:
		printUsage()
		os.Exit(1)
	}
}

func runCreate() {
	if len(os.Args) < 3 {
		log.Fatal("usage: migrate create <name>")
	}
	name := strings.Join(os.Args[2:], " ")
	files, err := migrate.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		fmt.Printf("created %s\n", f)
	}
}

func runSteps(dsn string) {
	if len(os.Args) < 3 {
		log.Fatal("usage: migrate steps <n>")
	}
	n, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	fatalIf(migrate.Steps(dsn, n))
}

func runVersion(dsn string) {
	version, dirty, err := migrate.Version(dsn)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("version: %d, dirty: %v\n", version, dirty)
}

func fatalIf(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {
	fmt.Println(`Usage:
  migrate create <name>   Create a new migration pair (up/down)
  migrate up              Apply all pending migrations
  migrate down            Rollback all migrations
  migrate steps <n>       Apply n migrations (negative to rollback)
  migrate version         Show current migration version`)
}
