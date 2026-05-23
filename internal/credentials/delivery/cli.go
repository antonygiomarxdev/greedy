package delivery

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/crypto"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func CredentialCommand(ctx context.Context, args []string) {
	masterPassword := os.Getenv("GREEDY_MASTER_PASSWORD")
	if masterPassword == "" {
		fmt.Fprintln(os.Stderr, "error: GREEDY_MASTER_PASSWORD environment variable is required")
		os.Exit(1)
	}
	masterKey := crypto.DeriveKey(masterPassword, nil)

	dataDir := os.Getenv("GREEDY_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = home + "/.greedy"
	}

	database, err := db.Open(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close(database)

	if err := db.RunMigrations(database); err != nil {
		fmt.Fprintf(os.Stderr, "error running migrations: %v\n", err)
		os.Exit(1)
	}

	store := credentials.NewSQLiteStore(database)

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: greedy credential <set|get|list|delete> [flags]")
		os.Exit(1)
	}

	switch args[0] {
	case "set":
		credentialSet(ctx, store, &masterKey, args[1:])
	case "get":
		credentialGet(ctx, store, &masterKey, args[1:])
	case "list":
		credentialList(ctx, store, args[1:])
	case "delete":
		credentialDelete(ctx, store, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown credential subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func credentialSet(ctx context.Context, store *credentials.SQLiteStore, masterKey *[32]byte, args []string) {
	fset := flag.NewFlagSet("credential set", flag.ExitOnError)
	exchange := fset.String("exchange", "", "exchange name (binance, coinbase)")
	label := fset.String("label", "default", "credential label")
	apiKey := fset.String("api-key", "", "API key")
	apiSecret := fset.String("api-secret", "", "API secret")
	passphrase := fset.String("passphrase", "", "passphrase (Coinbase only)")
	apiKeyEnv := fset.String("api-key-env", "", "env var containing API key")
	apiSecretEnv := fset.String("api-secret-env", "", "env var containing API secret")
	if err := fset.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	key := *apiKey
	if key == "" && *apiKeyEnv != "" {
		key = os.Getenv(*apiKeyEnv)
	}
	secret := *apiSecret
	if secret == "" && *apiSecretEnv != "" {
		secret = os.Getenv(*apiSecretEnv)
	}

	if *exchange == "" || key == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "error: --exchange, --api-key (or --api-key-env), and --api-secret (or --api-secret-env) are required")
		os.Exit(1)
	}

	cred := credentials.Credential{
		Exchange:   shared.ExchangeProvider(*exchange),
		Label:      *label,
		APIKey:     key,
		APISecret:  secret,
		Passphrase: *passphrase,
	}

	if err := store.Set(ctx, cred, masterKey); err != nil {
		fmt.Fprintf(os.Stderr, "error setting credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("credential stored: exchange=%s label=%s\n", *exchange, *label)
}

func credentialGet(ctx context.Context, store *credentials.SQLiteStore, masterKey *[32]byte, args []string) {
	fset := flag.NewFlagSet("credential get", flag.ExitOnError)
	exchange := fset.String("exchange", "", "exchange name")
	label := fset.String("label", "default", "credential label")
	if err := fset.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}
	if *exchange == "" {
		fmt.Fprintln(os.Stderr, "error: --exchange is required")
		os.Exit(1)
	}

	cred, err := store.Get(ctx, shared.ExchangeProvider(*exchange), *label, masterKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("exchange:   %s\n", cred.Exchange)
	fmt.Printf("label:      %s\n", cred.Label)
	fmt.Printf("api_key:    %s\n", cred.APIKey)
	fmt.Printf("api_secret: %s\n", cred.APISecret)
	if cred.Passphrase != "" {
		fmt.Printf("passphrase: %s\n", cred.Passphrase)
	}
}

func credentialList(ctx context.Context, store *credentials.SQLiteStore, _ []string) {
	metas, err := store.List(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing credentials: %v\n", err)
		os.Exit(1)
	}
	if len(metas) == 0 {
		fmt.Println("no credentials stored")
		return
	}
	for _, m := range metas {
		fmt.Printf("%s/%s\n", m.Exchange, m.Label)
	}
}

func credentialDelete(ctx context.Context, store *credentials.SQLiteStore, args []string) {
	fset := flag.NewFlagSet("credential delete", flag.ExitOnError)
	exchange := fset.String("exchange", "", "exchange name")
	label := fset.String("label", "default", "credential label")
	if err := fset.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}
	if *exchange == "" {
		fmt.Fprintln(os.Stderr, "error: --exchange is required")
		os.Exit(1)
	}

	if err := store.Delete(ctx, shared.ExchangeProvider(*exchange), *label); err != nil {
		fmt.Fprintf(os.Stderr, "error deleting credential: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("credential deleted: exchange=%s label=%s\n", *exchange, *label)
}
