package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Login via QR code",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := newApp(ctx, flags)
			if err != nil {
				return err
			}
			defer a.Close()

			if a.IsAuthenticated() {
				return fmt.Errorf("already authenticated")
			}

			if err := a.LoginWithQR(ctx); err != nil {
				return err
			}

			fmt.Println("Login successful!")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := newApp(ctx, flags)
			if err != nil {
				return err
			}
			defer a.Close()

			if a.IsAuthenticated() {
				user, err := a.GetUserInfo(ctx)
				if err == nil {
					fmt.Printf("Authenticated as: %s\n", user.Nickname)
				} else {
					fmt.Println("Authenticated: yes")
				}
			} else {
				fmt.Println("Authenticated: no")
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Logout and clear session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := newApp(ctx, flags)
			if err != nil {
				return err
			}
			defer a.Close()

			// Clear cookies
			fmt.Println("Logged out successfully")
			return nil
		},
	})

	return cmd
}
