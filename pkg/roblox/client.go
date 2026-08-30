package roblox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserProfile holds full aggregated information about a Roblox user.
type UserProfile struct {
	ID             int64        `json:"id"`
	Username       string       `json:"username"`
	DisplayName    string       `json:"displayName"`
	Description    string       `json:"description"`
	Created        time.Time    `json:"created"`
	IsBanned       bool         `json:"isBanned"`
	ExternalAppDisplayName string `json:"externalAppDisplayName,omitempty"`

	// Stats
	FollowersCount int64        `json:"followersCount"`
	FollowingCount int64        `json:"followingCount"`
	FriendsCount   int64        `json:"friendsCount"`

	// Avatars
	AvatarHeadshot string       `json:"avatarHeadshot"`
	AvatarBust     string       `json:"avatarBust"`
	AvatarFull     string       `json:"avatarFull"`

	// Presence
	Presence       *PresenceInfo `json:"presence,omitempty"`

	// Groups & Badges summary
	Groups         []GroupMember `json:"groups,omitempty"`
	Badges         []BadgeInfo   `json:"badges,omitempty"`
}

type PresenceInfo struct {
	UserPresenceType int    `json:"userPresenceType"` // 0: Offline, 1: Online, 2: InGame, 3: InStudio
	LastOnline       string `json:"lastOnline"`
	LastLocation     string `json:"lastLocation"`
	PlaceID          int64  `json:"placeId,omitempty"`
	GameID           string `json:"gameId,omitempty"`
}

type GroupMember struct {
	Group struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
	Role struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Rank int    `json:"rank"`
	} `json:"role"`
}

type BadgeInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconImageID int64  `json:"iconImageId"`
}

// Client provides high performance access to Roblox APIs.
type Client struct {
	httpClient *http.Client
	cache      sync.Map // simple concurrent cache
	cacheTTL   time.Duration
}

type cacheItem struct {
	profile   *UserProfile
	fetchedAt time.Time
}

// NewClient constructs a client with custom tuned HTTP Transport for concurrency & speed.
func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		cacheTTL: 2 * time.Minute,
	}
}

// ResolveUserID resolves an input string (username or ID string) to a numeric Roblox User ID.
func (c *Client) ResolveUserID(ctx context.Context, query string) (int64, string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, "", fmt.Errorf("query cannot be empty")
	}

	// Check if already numeric ID
	if id, err := strconv.ParseInt(query, 10, 64); err == nil && id > 0 {
		return id, query, nil
	}

	// Otherwise lookup username via Roblox API
	reqBody := fmt.Sprintf(`{"usernames":["%s"],"excludeBannedUsers":false}`, query)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://users.roblox.com/v1/usernames/users", strings.NewReader(reqBody))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("users API returned status %d", resp.StatusCode)
	}

	var res struct {
		Data []struct {
			RequestedUsername string `json:"requestedUsername"`
			ID                int64  `json:"id"`
			Name              string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, "", err
	}

	if len(res.Data) == 0 {
		return 0, "", fmt.Errorf("user '%s' not found", query)
	}

	return res.Data[0].ID, res.Data[0].Name, nil
}

// LookupUser performs concurrent requests to fetch all available details for a user.
func (c *Client) LookupUser(ctx context.Context, query string) (*UserProfile, error) {
	queryKey := strings.ToLower(strings.TrimSpace(query))
	if val, ok := c.cache.Load(queryKey); ok {
		item := val.(cacheItem)
		if time.Since(item.fetchedAt) < c.cacheTTL {
			return item.profile, nil
		}
	}

	userID, resolvedName, err := c.ResolveUserID(ctx, query)
	if err != nil {
		return nil, err
	}

	// Check cache by numeric ID as well
	idKey := fmt.Sprintf("id:%d", userID)
	if val, ok := c.cache.Load(idKey); ok {
		item := val.(cacheItem)
		if time.Since(item.fetchedAt) < c.cacheTTL {
			return item.profile, nil
		}
	}

	profile := &UserProfile{
		ID:       userID,
		Username: resolvedName,
	}

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	setErr := func(e error) {
		if e != nil {
			errOnce.Do(func() {
				firstErr = e
			})
		}
	}

	// 1. Fetch User Info details (users.roblox.com/v1/users/{id})
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://users.roblox.com/v1/users/%d", userID)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			setErr(err)
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			setErr(err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var data struct {
				Description            string    `json:"description"`
				Created                time.Time `json:"created"`
				IsBanned               bool      `json:"isBanned"`
				ExternalAppDisplayName string    `json:"externalAppDisplayName"`
				Name                   string    `json:"name"`
				DisplayName            string    `json:"displayName"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
				profile.Description = data.Description
				profile.Created = data.Created
				profile.IsBanned = data.IsBanned
				profile.ExternalAppDisplayName = data.ExternalAppDisplayName
				profile.Username = data.Name
				profile.DisplayName = data.DisplayName
			}
		}
	}()

	// 2. Fetch Followers Count
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://friends.roblox.com/v1/users/%d/followers/count", userID)
		if data, err := c.getJsonMap(ctx, url); err == nil {
			if count, ok := data["count"].(float64); ok {
				profile.FollowersCount = int64(count)
			}
		}
	}()

	// 3. Fetch Following Count
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://friends.roblox.com/v1/users/%d/followings/count", userID)
		if data, err := c.getJsonMap(ctx, url); err == nil {
			if count, ok := data["count"].(float64); ok {
				profile.FollowingCount = int64(count)
			}
		}
	}()

	// 4. Fetch Friends Count
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://friends.roblox.com/v1/users/%d/friends/count", userID)
		if data, err := c.getJsonMap(ctx, url); err == nil {
			if count, ok := data["count"].(float64); ok {
				profile.FriendsCount = int64(count)
			}
		}
	}()

	// 5. Fetch Avatars (Headshot, Bust, Full Body) in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		headshotURL := fmt.Sprintf("https://thumbnails.roblox.com/v1/users/avatar-headshot?userIds=%d&size=420x420&format=Png&isCircular=false", userID)
		bustURL := fmt.Sprintf("https://thumbnails.roblox.com/v1/users/avatar-bust?userIds=%d&size=420x420&format=Png", userID)
		fullURL := fmt.Sprintf("https://thumbnails.roblox.com/v1/users/avatar?userIds=%d&size=720x720&format=Png", userID)

		var imgWg sync.WaitGroup
		imgWg.Add(3)

		go func() {
			defer imgWg.Done()
			profile.AvatarHeadshot = c.extractThumbnail(ctx, headshotURL)
		}()
		go func() {
			defer imgWg.Done()
			profile.AvatarBust = c.extractThumbnail(ctx, bustURL)
		}()
		go func() {
			defer imgWg.Done()
			profile.AvatarFull = c.extractThumbnail(ctx, fullURL)
		}()

		imgWg.Wait()
	}()

	// 6. Fetch Presence Info
	wg.Add(1)
	go func() {
		defer wg.Done()
		reqBody := fmt.Sprintf(`{"userIds": [%d]}`, userID)
		req, err := http.NewRequestWithContext(ctx, "POST", "https://presence.roblox.com/v1/presence/users", strings.NewReader(reqBody))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var res struct {
				UserPresences []PresenceInfo `json:"userPresences"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && len(res.UserPresences) > 0 {
				profile.Presence = &res.UserPresences[0]
			}
		}
	}()

	// 7. Fetch User Groups
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://groups.roblox.com/v2/users/%d/groups/roles", userID)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var res struct {
				Data []GroupMember `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
				profile.Groups = res.Data
			}
		}
	}()

	// 8. Fetch User Badges
	wg.Add(1)
	go func() {
		defer wg.Done()
		url := fmt.Sprintf("https://badges.roblox.com/v1/users/%d/badges?limit=10&sortOrder=Desc", userID)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var res struct {
				Data []BadgeInfo `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
				profile.Badges = res.Data
			}
		}
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Cache result
	item := cacheItem{profile: profile, fetchedAt: time.Now()}
	c.cache.Store(queryKey, item)
	c.cache.Store(idKey, item)

	return profile, nil
}

func (c *Client) getJsonMap(ctx context.Context, targetURL string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) extractThumbnail(ctx context.Context, thumbnailURL string) string {
	data, err := c.getJsonMap(ctx, thumbnailURL)
	if err != nil {
		return ""
	}
	if dataArr, ok := data["data"].([]interface{}); ok && len(dataArr) > 0 {
		if first, ok := dataArr[0].(map[string]interface{}); ok {
			if imageUrl, ok := first["imageUrl"].(string); ok {
				return imageUrl
			}
		}
	}
	return ""
}
