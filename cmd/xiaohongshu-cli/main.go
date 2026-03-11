package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/saulyip/auto-xiaohongshu/internal/config"
	"github.com/saulyip/auto-xiaohongshu/internal/app"
	"github.com/saulyip/auto-xiaohongshu/internal/out"
)

var version = "0.1.0"

type rootFlags struct {
	storeDir string
	asJSON   bool
	headless bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	var flags rootFlags

	rootCmd := &cobra.Command{
		Use:           "xiaohongshu-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("xiaohongshu-cli {{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flags.storeDir, "store", "~/.xiaohongshu-cli", "store directory for session data")
	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "output JSON instead of human-readable text")
	rootCmd.PersistentFlags().BoolVar(&flags.headless, "headless", true, "run browser in headless mode")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAuthCmd(&flags))
	rootCmd.AddCommand(newFeedCmd(&flags))
	rootCmd.AddCommand(newSearchCmd(&flags))

	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		_ = out.WriteError(os.Stderr, flags.asJSON, err)
		return err
	}
	return nil
}

func newApp(ctx context.Context, flags *rootFlags) (*app.App, error) {
	return app.New(app.Options{
		StoreDir: config.ExpandPath(flags.storeDir),
		Headless: flags.headless,
		JSON:     flags.asJSON,
	})
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("xiaohongshu-cli %s\n", version)
		},
	}
}
