package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/saulyip/auto-xiaohongshu/internal/api"
	"github.com/saulyip/auto-xiaohongshu/internal/config"
	"github.com/skip2/go-qrcode"
)

type Options struct {
	StoreDir string
	Headless bool
	JSON     bool
}

type App struct {
	opts Options
	mu   sync.Mutex
	browser   playwright.Browser
	page      playwright.Page
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.browser != nil {
		a.browser.Close()
	}
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

// LoginWithQR performs QR code login using Playwright
func (a *App) LoginWithQR(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("run playwright: %w", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(a.opts.Headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--no-sandbox",
		},
	})
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}

	// Navigate to Xiaohongshu
	_, err = page.Goto("https://www.xiaohongshu.com/", playwright.PageGotoOptions{
		Timeout: playwright.Float(60000),
	})
	if err != nil {
		return fmt.Errorf("goto xiaohongshu: %w", err)
	}

	// Wait a bit for page to load
	page.WaitForTimeout(5000)

	// Dismiss upgrade popup by clicking close button
	closeBtn, _ := page.QuerySelector(".close, [class*='close'], [class*='Close']")
	if closeBtn != nil {
		closeBtn.Click()
		page.WaitForTimeout(1000)
	}

	// Press Escape to dismiss any modal
	page.Keyboard().Press("Escape")
	page.WaitForTimeout(1000)

	// Take initial screenshot to debug
	page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(filepath.Join(a.opts.StoreDir, "debug1.png")),
	})

	// Check if already logged in
	avatar, _ := page.QuerySelector(".user-avatar")
	if avatar != nil {
		return a.extractCookiesFromPage(page)
	}

	// Try to find and display QR code in terminal
	// Try various selectors to find QR code image
	qrSelectors := []string{
		".login-qrcode img",
		"[class*='qrcode'] img",
		"[class*='qr-code'] img",
		"#qrcode",
		"canvas[class*='qr']",
	}

	var qrURL string
	for _, sel := range qrSelectors {
		qrImg, err := page.QuerySelector(sel)
		if err == nil && qrImg != nil {
			src, err := qrImg.GetAttribute("src")
			if err == nil && src != "" {
				qrURL = src
				break
			}
			// Try canvas - need to get data URL
			if sel == "canvas" {
				data, err := page.Evaluate("document.querySelector('" + sel + "').toDataURL()")
				if err == nil {
					qrURL = data.(string)
					break
				}
			}
		}
	}

	if qrURL != "" {
		// Generate terminal QR code
		if strings.HasPrefix(qrURL, "data:image") {
			qrData := strings.TrimPrefix(qrURL, "data:image/png;base64,")
			decoded, err := base64.StdEncoding.DecodeString(qrData)
			if err == nil {
				qr, err := qrcode.New(string(decoded), qrcode.Medium)
				if err == nil {
					fmt.Println(qr.ToSmallString(false))
					fmt.Println("\nPlease scan this QR code with your Xiaohongshu app")
				}
			}
		} else if strings.HasPrefix(qrURL, "http") {
			qr, err := qrcode.New(qrURL, qrcode.Medium)
			if err == nil {
				fmt.Println(qr.ToSmallString(false))
				fmt.Println("\nPlease scan this QR code with your Xiaohongshu app")
			}
		}
	}

	// If no QR code displayed in terminal, save screenshot
	if qrURL == "" {
		qrPath := filepath.Join(a.opts.StoreDir, "qrcode.png")
		page.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(qrPath),
			FullPage: playwright.Bool(true),
		})
		fmt.Println("Please scan the QR code in:", qrPath)
	}

	// Wait for login to complete - look for avatar or user info
	_, err = page.WaitForSelector(".user-avatar, [class*='user-info'], [class*='userAvatar']", playwright.PageWaitForSelectorOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(120000), // 2 minutes
	})
	if err != nil {
		// Take debug screenshot
		page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String(filepath.Join(a.opts.StoreDir, "debug2.png")),
		})
		return fmt.Errorf("wait for login: %w", err)
	}

	return a.extractCookiesFromPage(page)
}

func (a *App) extractCookiesFromPage(page playwright.Page) error {
	cookies, err := page.Context().Cookies()
	if err != nil {
		return fmt.Errorf("get cookies: %w", err)
	}

	var httpCookies []*http.Cookie
	for _, c := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Now().Add(time.Hour * 24 * 30), // 30 days
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
			SameSite: http.SameSite(0),
		})
	}

	a.cookies = httpCookies
	a.apiClient.SetCookies(httpCookies)
	a.saveCookies()

	return nil
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

