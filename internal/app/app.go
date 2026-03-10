package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/saulyip/auto-xiaohongshu/internal/api"
	"github.com/saulyip/auto-xiaohongshu/internal/config"
)

type Options struct {
	StoreDir string
	JSON     bool
}

type App struct {
	opts Options
	mu   sync.Mutex
	cookies   []*http.Cookie
	apiClient *api.Client
}

func New(opts Options) (*App, error) {
	opts.StoreDir = config.ExpandPath(opts.StoreDir)
	if err := config.EnsureStoreDir(opts.StoreDir); err != nil {
		return nil, fmt.Errorf("ensure store dir: %w", err)
	}

	a := &App{opts: opts}

	// Load existing cookies if available
	a.loadCookies()

	// Initialize API client with cookies
	a.apiClient = api.NewClient()

	return a, nil
}

func (a *App) Close() {
	// No browser to close in API-based approach
}

func (a *App) loadCookies() {
	cookieFile := filepath.Join(a.opts.StoreDir, "cookies.json")
	data, err := os.ReadFile(cookieFile)
	if err != nil {
		return
	}
	var cookies []*http.Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return
	}
	a.cookies = cookies
	a.apiClient.SetCookies(cookies)
}

func (a *App) saveCookies() {
	if a.cookies == nil {
		return
	}
	cookieFile := filepath.Join(a.opts.StoreDir, "cookies.json")
	data, err := json.Marshal(a.cookies)
	if err != nil {
		return
	}
	os.WriteFile(cookieFile, data, 0600)
}

func (a *App) IsAuthenticated() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.cookies) > 0 && a.apiClient.IsAuthenticated()
}

// LoginWithQR performs QR code login using the API
func (a *App) LoginWithQR(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Step 1: Create QR code
	qrResp, err := a.apiClient.CreateQRCode(ctx)
	if err != nil {
		return fmt.Errorf("create QR code: %w", err)
	}

	fmt.Printf("QR Code URL: %s\n", qrResp.URL)
	fmt.Printf("QR ID: %s\n", qrResp.QRID)
	fmt.Println("Please scan the QR code with your Xiaohongshu app...")

	// Step 2: Poll for login status
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(120 * time.Second) // 2 minutes timeout

	for {
		select {
		case <-timeout:
			return fmt.Errorf("login timeout")
		case <-ticker.C:
			status, err := a.apiClient.PollQRStatus(ctx, qrResp.QRID, qrResp.Code)
			if err != nil {
				continue
			}

			switch status.Code {
			case 0:
				// Login successful - get cookies from API
				cookies, err := a.apiClient.GetSessionCookies(ctx, status.UserID)
				if err != nil {
					return fmt.Errorf("get session cookies: %w", err)
				}

				a.cookies = cookies
				a.apiClient.SetCookies(cookies)
				a.saveCookies()
				return nil
			case 4:
				// Still waiting for scan
				fmt.Println("Waiting for scan...")
			case 5:
				// Scanned but not confirmed
				fmt.Println("Scanned! Please confirm on your phone...")
			default:
				fmt.Printf("Unknown status: %d - %s\n", status.Code, status.Message)
			}
		}
	}
}

// Feed returns the homepage feed
func (a *App) GetFeed(ctx context.Context) ([]api.FeedItem, error) {
	return a.apiClient.GetFeed(ctx)
}

// Search posts
func (a *App) Search(ctx context.Context, query string, filter api.SearchFilter, sort api.SearchSort) ([]api.SearchResult, error) {
	return a.apiClient.Search(ctx, query, filter, sort)
}

// GetUserInfo returns current user info
func (a *App) GetUserInfo(ctx context.Context) (*api.User, error) {
	return a.apiClient.GetUserInfo(ctx)
}

// Format output based on JSON flag
func (a *App) FormatOutput(v interface{}) string {
	if a.opts.JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		return string(data)
	}
	return fmt.Sprintf("%+v", v)
}

// ParseNoteID extracts note ID from various URL formats
func (a *App) ParseNoteID(input string) string {
	input = strings.TrimSpace(input)
	// Already just the ID
	if !strings.Contains(input, "/") {
		return input
	}
	// Try to extract from URL
	// Format: https://www.xiaohongshu.com/explore/xxx
	parts := strings.Split(input, "/")
	last := parts[len(parts)-1]
	// Remove query params
	if idx := strings.Index(last, "?"); idx != -1 {
		last = last[:idx]
	}
	return last
}
