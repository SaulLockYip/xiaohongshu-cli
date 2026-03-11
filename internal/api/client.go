package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles API requests to Xiaohongshu
type Client struct {
	cookies   []*http.Cookie
	httpClient *http.Client
	baseURL   string
	seed      int64
}

// NewClient creates a new Xiaohongshu API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://edith.xiaohongshu.com",
		seed:    time.Now().UnixNano(),
	}
}

func (c *Client) SetCookies(cookies []*http.Cookie) {
	c.cookies = cookies
}

// IsAuthenticated checks if the client has valid authentication
func (c *Client) IsAuthenticated() bool {
	if len(c.cookies) == 0 {
		return false
	}
	for _, cookie := range c.cookies {
		if cookie.Name == "a1" {
			return cookie.Value != ""
		}
	}
	return false
}

func (c *Client) buildRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add cookies
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://www.xiaohongshu.com")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")

	// Add CSRF token from cookies
	for _, cookie := range c.cookies {
		if cookie.Name == "csrf_token" {
			req.Header.Set("x-csrf-token", cookie.Value)
			break
		}
	}

	return req, nil
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DS parameter generation - this is critical for API requests
func (c *Client) generateDS(page, api string) string {
	// Simplified DS generation - real implementation would need more complex logic
	// This is based on reverse engineering Xiaohongshu's web app
	now := time.Now().UnixMilli()
	randNum := rand.Int63n(1000000)

	// Build the sign string
	signStr := fmt.Sprintf("{%s}{%s}{%d}{%d}", page, api, now, randNum)

	// Simple XOR cipher as placeholder - real implementation uses more complex encryption
	ds := fmt.Sprintf("%d,%d,%s", now, randNum, signStr[:8])

	return base64.StdEncoding.EncodeToString([]byte(ds))
}

// SearchFilter defines the type of content to search
type SearchFilter string

const (
	FilterAll      SearchFilter = "all"       // 全部
	FilterImage    SearchFilter = "image"     // 图文
	FilterVideo    SearchFilter = "video"     // 视频
	FilterUser     SearchFilter = "user"      // 用户
)

// SearchSort defines how to sort search results
type SearchSort string

const (
	SortComprehensive SearchSort = "general"  // 综合
	SortPopular       SearchSort = "hot"       // 最热
	SortRecent        SearchSort = "time"      // 最新
)

// SearchResult represents a search result item
type SearchResult struct {
	NoteID     string `json:"note_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Author     Author `json:"author"`
	Likes      int    `json:"likes"`
	Comments   int    `json:"comments"`
	Shares     int    `json:"shares"`
	CoverImage string `json:"cover_image"`
	Type       string `json:"type"`
}

// Author represents a user
type Author struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// FeedItem represents an item in the homepage feed
type FeedItem struct {
	NoteID     string `json:"note_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Author     Author `json:"author"`
	Images     []string `json:"images"`
	Likes      int    `json:"likes"`
	Comments   int    `json:"comments"`
	Collects   int    `json:"collects"`
	Timestamp  int64  `json:"timestamp"`
}

// Search performs a search query
func (c *Client) Search(ctx context.Context, query string, filter SearchFilter, sort SearchSort) ([]SearchResult, error) {
	apiPath := fmt.Sprintf("/api/sns/web/v1/search/notes?keyword=%s&search_id=&sort=%s&filter=%s&page=1&page_size=20",
		url.QueryEscape(query), sort, filter)

	// Generate DS parameter
	ds := c.generateDS("search", apiPath)

	req, err := c.buildRequest(ctx, "GET", apiPath+"&ds="+url.QueryEscape(ds), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search failed: status %d", resp.StatusCode)
	}

	var result struct {
		Code    int            `json:"code"`
		Success bool           `json:"success"`
		Msg     string         `json:"msg"`
		Data    SearchResponse `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("search error: %s", result.Msg)
	}

	return result.Data.Items, nil
}

type SearchResponse struct {
	Items []SearchResult `json:"items"`
}

// GetFeed fetches the homepage feed
func (c *Client) GetFeed(ctx context.Context) ([]FeedItem, error) {
	apiPath := "/api/sns/web/v1/homefeed"

	ds := c.generateDS("feed", apiPath)

	req, err := c.buildRequest(ctx, "POST", apiPath, map[string]interface{}{
		"xsec_source": "pc_feed",
		"xsec_token":  "AB",
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-dsig", ds)

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int        `json:"code"`
		Success bool       `json:"success"`
		Msg     string     `json:"msg"`
		Data    FeedResponse `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("feed error: %s", result.Msg)
	}

	return result.Data.Items, nil
}

type FeedResponse struct {
	Items []FeedItem `json:"items"`
}

// Post represents a post
type Post struct {
	NoteID     string   `json:"note_id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Author     Author   `json:"author"`
	Images     []string `json:"images"`
	Video      string   `json:"video"`
	Likes      int      `json:"likes"`
	Comments   int      `json:"comments"`
	Collects   int      `json:"collects"`
	Shares     int      `json:"shares"`
	Timestamp  int64    `json:"timestamp"`
	Location   string   `json:"location"`
}

// GetPost fetches a specific post
func (c *Client) GetPost(ctx context.Context, noteID string) (*Post, error) {
	apiPath := fmt.Sprintf("/api/sns/web/v1/notes/%s", noteID)

	req, err := c.buildRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Data    Post   `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("get post error: %s", result.Msg)
	}

	return &result.Data, nil
}

// LikePost likes a post
func (c *Client) LikePost(ctx context.Context, noteID string) error {
	apiPath := "/api/sns/web/v1/note/like"

	req, err := c.buildRequest(ctx, "POST", apiPath, map[string]string{
		"note_id": noteID,
	})
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("like failed: %s", result.Msg)
	}

	return nil
}

// UnlikePost unlikes a post
func (c *Client) UnlikePost(ctx context.Context, noteID string) error {
	apiPath := "/api/sns/web/v1/note/unlike"

	req, err := c.buildRequest(ctx, "POST", apiPath, map[string]string{
		"note_id": noteID,
	})
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("unlike failed: %s", result.Msg)
	}

	return nil
}

// Comment represents a comment
type Comment struct {
	CommentID  string   `json:"comment_id"`
	Content    string   `json:"content"`
	Author     Author   `json:"author"`
	Likes      int      `json:"likes"`
	Replies    int      `json:"replies"`
	Timestamp  int64    `json:"timestamp"`
	Location   string   `json:"location"`
}

// CommentsResponse represents the response from getting comments
type CommentsResponse struct {
	Comments []Comment `json:"comments"`
	Cursor   string    `json:"cursor"`
	HasMore  bool      `json:"has_more"`
}

// GetComments fetches comments for a post
func (c *Client) GetComments(ctx context.Context, noteID, cursor string) (*CommentsResponse, error) {
	apiPath := fmt.Sprintf("/api/sns/web/v1/notes/%s/comments?cursor=%s&top_comment_id=&image_formats=jpg,webp,avif", noteID, cursor)

	req, err := c.buildRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int             `json:"code"`
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Data    CommentsResponse `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("get comments error: %s", result.Msg)
	}

	return &result.Data, nil
}

// CommentPost adds a comment to a post
func (c *Client) CommentPost(ctx context.Context, noteID, content string) error {
	apiPath := "/api/sns/web/v1/comment/publish"

	req, err := c.buildRequest(ctx, "POST", apiPath, map[string]string{
		"note_id": noteID,
		"content": content,
	})
	if err != nil {
		return err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("comment failed: %s", result.Msg)
	}

	return nil
}

// Publish publishes a new post
func (c *Client) Publish(ctx context.Context, title, content string, images []string) (string, error) {
	apiPath := "/api/sns/web/v1/notes"

	type ImageInfo struct {
		FileID string `json:"file_id"`
	}

	type PublishReq struct {
		Title   string       `json:"title"`
		Content string       `json:"content"`
		Images  []ImageInfo  `json:"images"`
		Type    string       `json:"type"`
	}

	// Upload images first (simplified - real implementation would upload to image service)
	var imgInfos []ImageInfo
	for range images {
		imgInfos = append(imgInfos, ImageInfo{FileID: "placeholder"})
	}

	reqBody := PublishReq{
		Title:   title,
		Content: content,
		Images:  imgInfos,
		Type:    "normal",
	}

	req, err := c.buildRequest(ctx, "POST", apiPath, reqBody)
	if err != nil {
		return "", err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Data    struct {
			NoteID string `json:"note_id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("publish failed: %s", result.Msg)
	}

	return result.Data.NoteID, nil
}

// User represents a user
type User struct {
	UserID      string `json:"user_id"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Desc        string `json:"desc"`
	Follows     int    `json:"follows"`
	Fans        int    `json:"fans"`
	Likes       int    `json:"likes"`
	IPLocation  string `json:"ip_location"`
}

// GetUserInfo fetches current user info
func (c *Client) GetUserInfo(ctx context.Context) (*User, error) {
	apiPath := "/api/sns/web/v1/user/me"

	req, err := c.buildRequest(ctx, "GET", apiPath, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Data    User   `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("get user info error: %s", result.Msg)
	}

	return &result.Data, nil
}

// QRCodeResponse represents the QR code creation response
type QRCodeResponse struct {
	URL   string `json:"url"`
	QRID  string `json:"qr_id"`
	Code  string `json:"code"`
}

// QRStatus represents the QR code polling status
type QRStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	UserID  string `json:"userId"`
}

// CreateQRCode creates a new QR code for login
func (c *Client) CreateQRCode(ctx context.Context) (*QRCodeResponse, error) {
	// First, visit the main page to get initial cookies
	initialReq, _ := http.NewRequestWithContext(ctx, "GET", "https://www.xiaohongshu.com/", nil)
	initialReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	initialResp, err := c.httpClient.Do(initialReq)
	if err == nil {
		// Collect cookies from initial request
		for _, cookie := range initialResp.Cookies() {
			c.cookies = append(c.cookies, cookie)
		}
		initialResp.Body.Close()
	}

	// Now create the QR code request with cookies
	body := map[string]interface{}{
		"qr_type": 1,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/sns/web/v1/login/qrcode/create", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Build cookie header manually
	var cookieParts []string
	for _, cookie := range c.cookies {
		cookieParts = append(cookieParts, cookie.Name+"="+cookie.Value)
	}
	cookieHeader := strings.Join(cookieParts, "; ")

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://www.xiaohongshu.com")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("xsecappid", "xhs-pc-web")
	req.Header.Set("x-t", fmt.Sprintf("%d", time.Now().UnixMilli()))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code    int             `json:"code"`
		Success bool            `json:"success"`
		Msg     string         `json:"msg"`
		Data    QRCodeResponse `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w, body: %s", err, string(respBody))
	}

	if !result.Success {
		return nil, fmt.Errorf("create QR code failed: code=%d, msg=%s, body=%s", result.Code, result.Msg, string(respBody))
	}

	return &result.Data, nil
}

// PollQRStatus polls for QR code login status
func (c *Client) PollQRStatus(ctx context.Context, qrID, code string) (*QRStatus, error) {
	// Create request without auth headers for polling (public endpoint)
	body := map[string]string{
		"qrId": qrID,
		"code": code,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/qrcode/userinfo", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://www.xiaohongshu.com")
	req.Header.Set("Referer", "https://www.xiaohongshu.com/")
	req.Header.Set("xsecappid", "xhs-pc-web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code    int       `json:"code"`
		Success bool      `json:"success"`
		Data    struct {
			Result    QRStatus `json:"result"`
			CodeStatus int     `json:"codeStatus"`
			UserID    string   `json:"userId"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	status := &result.Data.Result
	status.Code = result.Data.CodeStatus
	status.UserID = result.Data.UserID

	return status, nil
}

// GetSessionCookies retrieves session cookies after successful QR login
// This requires making a request to get the cookies set during login
func (c *Client) GetSessionCookies(ctx context.Context, userID string) ([]*http.Cookie, error) {
	// Navigate to xiaohongshu.com to get cookies from the response
	// The cookies should be automatically set in the response
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.xiaohongshu.com/", nil)
	if err != nil {
		return nil, err
	}

	// Set the cookies we might have from previous requests
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Extract cookies from response
	var cookies []*http.Cookie
	for _, c := range resp.Cookies() {
		cookies = append(cookies, c)
	}

	// We need to get the full session - typically this requires visiting the page
	// For now, return the existing cookies which should include a1, web_session, etc.
	if len(cookies) == 0 {
		cookies = c.cookies
	}

	return cookies, nil
}
