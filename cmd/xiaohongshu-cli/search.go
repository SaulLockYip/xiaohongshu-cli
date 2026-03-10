package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/saulyip/auto-xiaohongshu/internal/api"
	"github.com/saulyip/auto-xiaohongshu/internal/out"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var filter string
	var sort string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search posts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := newApp(ctx, flags)
			if err != nil {
				return err
			}
			defer a.Close()

			query := args[0]

			searchFilter := api.SearchFilter(filter)
			searchSort := api.SearchSort(sort)

			results, err := a.Search(ctx, query, searchFilter, searchSort)
			if err != nil {
				return out.WriteError(nil, flags.asJSON, err)
			}

			if flags.asJSON {
				return out.WriteJSON(cmd.OutOrStdout(), results)
			}

			fmt.Printf("Search results for: %s\n", query)
			fmt.Printf("Filter: %s, Sort: %s\n\n", filter, sort)

			for i, r := range results {
				fmt.Printf("%d. %s\n", i+1, r.Title)
				if r.Content != "" {
					fmt.Printf("   %s\n", truncate(r.Content, 80))
				}
				fmt.Printf("   👤 %s | 👍 %d | 💬 %d\n", r.Author.Nickname, r.Likes, r.Comments)
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&filter, "filter", "all", "Filter: all, image, video, user")
	cmd.Flags().StringVar(&sort, "sort", "general", "Sort: general (comprehensive), hot (most popular), time (recent)")

	return cmd
}
