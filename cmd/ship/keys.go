package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/secrets"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage your encryption key",
	Long:  `Generates and stores the private key used to encrypt .env.encrypted files.`,
}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate [keychain|file|env]",
	Short: "Generate a new age key",
	Long: `Choose where to store the private key:

  keychain  macOS Keychain (syncs via iCloud, recommended for macOS)
  file      ~/.config/ship/age-key.txt (cross-platform)
  env       You set SHIP_AGE_KEY yourself (CI, 1Password injection)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := "keychain"
		if len(args) > 0 {
			store = args[0]
		}
		var ks secrets.KeyStore
		switch store {
		case "keychain":
			ks = secrets.KeyStoreKeychain
		case "file":
			ks = secrets.KeyStoreFile
		case "env":
			ks = secrets.KeyStoreEnv
		default:
			return fmt.Errorf("unknown store %q — use: keychain, file, or env", store)
		}
		id, err := secrets.Generate(ks)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Key generated (%s)\n", store)
		if store == "env" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Set this in your shell: export SHIP_AGE_KEY=%s\n", id.String())
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Next: ship secrets set KEY=val\n")
		return nil
	},
}

var keysStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show where your key is stored",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := secrets.DetectKeyStore()
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "No key found.")
			fmt.Fprintln(cmd.OutOrStdout(), "  Generate: ship keys generate keychain")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Store: %s\n", store)
		return nil
	},
}

func initKeys() {
	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysStatusCmd)
}
