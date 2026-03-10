package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/saulyip/auto-xiaohongshu/internal/out"
)

func newFeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed",
		Short: "View homepage feed",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List homepage feed",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := newApp(ctx, flags)
			if err != nil {
				return err
			}
			defer a.Close()

			if !a.IsAuthenticated() {
				return out.WriteError(nil, flags.asJSON, fmt.Errorf("not authenticated, run 'auth login' first"))
			}

			items, err := a.GetFeed(ctx)
			if err != nil {
				return out.WriteError(nil, flags.asJSON, err)
			}

			if flags.asJSON {
				return out.WriteJSON(cmd.OutOrStdout(), items)
			}

			for i, item := range items {
				fmt.Printf("%d. %s\n", i+1, item.Title)
				if item.Content != "" {
					fmt.Printf("   %s\n", truncate(item.Content, 100))
				}
				fmt.Printf("   👤 %s | 👍 %d | 💬 %d\n", item.Author.Nickname, item.Likes, item.Comments)
				fmt.Println()
			}
			return nil
		},
	})

	return cmd
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
