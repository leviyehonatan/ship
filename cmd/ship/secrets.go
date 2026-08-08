package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/secrets"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets (like fly secrets)",
	Long: `Manages encrypted secrets stored in .env.encrypted.
Works like fly secrets — set, unset, list keys directly.
No plaintext .env file needed.

Private key: ~/.config/ship/age-key.txt
Encrypted secrets: .env.encrypted (safe to commit)`,
}

var secretsSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE",
	Short: "Set a secret",
	Long:  `Encrypts and stores a secret in .env.encrypted. Creates the file if it doesn't exist.`,
	Example: `  ship secrets set DATABASE_URL=postgresql://...
  ship secrets set COUCHDB_PASSWORD=secret123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("use KEY=VALUE format (e.g. COUCHDB_PASSWORD=secret)")
		}
		path := ".env.encrypted"
		if pathFlag, _ := cmd.Flags().GetString("file"); pathFlag != "" {
			path = pathFlag
		}
		if err := secrets.Set(path, parts[0], parts[1]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ %s set in %s\n", parts[0], path)
		return nil
	},
}

var secretsUnsetCmd = &cobra.Command{
	Use:   "unset KEY",
	Short: "Remove a secret",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".env.encrypted"
		if pathFlag, _ := cmd.Flags().GetString("file"); pathFlag != "" {
			path = pathFlag
		}
		if err := secrets.Unset(path, args[0]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ %s removed from %s\n", args[0], path)
		return nil
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret keys (values hidden)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".env.encrypted"
		if pathFlag, _ := cmd.Flags().GetString("file"); pathFlag != "" {
			path = pathFlag
		}
		keys, err := secrets.List(path)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No secrets set.")
			return nil
		}
		for _, k := range keys {
			fmt.Fprintln(cmd.OutOrStdout(), k)
		}
		return nil
	},
}

var secretsImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import an existing .env file into encrypted storage",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".env"
		if len(args) > 0 {
			path = args[0]
		}
		if err := secrets.EncryptFile(path); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Imported %s → %s.encrypted\n", path, path)
		fmt.Fprintln(cmd.OutOrStdout(), "  Add .env to .gitignore, commit .env.encrypted")
		return nil
	},
}

var secretsShowCmd = &cobra.Command{
	Use:   "show [key]",
	Short: "Show a secret value or all values",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".env.encrypted"
		if pathFlag, _ := cmd.Flags().GetString("file"); pathFlag != "" {
			path = pathFlag
		}
		m, err := secrets.ReadAll(path)
		if err != nil {
			return err
		}
		if len(args) > 0 {
			if v, ok := m[args[0]]; ok {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			} else {
				return fmt.Errorf("key %q not found", args[0])
			}
			return nil
		}
		for k, v := range m {
			fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, v)
		}
		return nil
	},
}

func initSecrets() {
	secretsSetCmd.Flags().String("file", ".env.encrypted", "Secrets file path")
	secretsUnsetCmd.Flags().String("file", ".env.encrypted", "Secrets file path")
	secretsListCmd.Flags().String("file", ".env.encrypted", "Secrets file path")
	secretsShowCmd.Flags().String("file", ".env.encrypted", "Secrets file path")

	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsUnsetCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsImportCmd)
	secretsCmd.AddCommand(secretsShowCmd)
	secretsCmd.AddCommand(keysExportCmd)
	secretsCmd.AddCommand(keysImportCmd)
}

var keysExportCmd = &cobra.Command{
	Use:   "export-key",
	Short: "Print the age private key (for syncing across devices)",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := secrets.EnsureKeys()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(config.StateDir(), "age-key.txt"))
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
}

var keysImportCmd = &cobra.Command{
	Use:   "import-key",
	Short: "Import an age private key (paste from another device)",
	Long:  "Reads an age private key from stdin and stores it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		key := strings.TrimSpace(string(data))
		if key == "" || !strings.HasPrefix(key, "AGE-SECRET-KEY-") {
			return fmt.Errorf("invalid age key — should start with AGE-SECRET-KEY-")
		}
		os.MkdirAll(config.StateDir(), 0700)
		if err := os.WriteFile(filepath.Join(config.StateDir(), "age-key.txt"), []byte(key), 0600); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Key imported. Test with: ship secrets list")
		return nil
	},
}
