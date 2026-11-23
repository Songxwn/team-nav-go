// Package model defines the persisted entities shared by the store, HTTP API
// and the embedded frontend.
package model

import "strings"

// Category is a node of the navigation tree. The tree is limited to three
// levels by the API, but the schema itself supports arbitrary nesting.
type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	ParentID    int64  `json:"parentId"`
	Sort        int    `json:"sort"`
	Visible     bool   `json:"visible"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`

	Children  []*Category `json:"children,omitempty"`
	CardCount int         `json:"cardCount"`
}

// Card is a navigation entry. Status is either "approved" or "pending".
type Card struct {
	ID          int64    `json:"id"`
	CategoryID  int64    `json:"categoryId"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	IconType    string   `json:"iconType"` // image | url | text | emoji
	IconValue   string   `json:"iconValue"`
	Tags        []string `json:"tags"`
	Sort        int      `json:"sort"`
	Status      string   `json:"status"`
	Clicks      int64    `json:"clicks"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

// Normalize trims fields and makes tags safe for persistence.
func (c *Card) Normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.URL = strings.TrimSpace(c.URL)
	c.Description = strings.TrimSpace(c.Description)
	if c.IconType == "" {
		c.IconType = "text"
	}
	if c.Status != "pending" {
		c.Status = "approved"
	}
	seen := make(map[string]struct{}, len(c.Tags))
	tags := make([]string, 0, len(c.Tags))
	for _, tag := range c.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
		if len(tags) == 6 {
			break
		}
	}
	c.Tags = tags
}

// User is an account that can sign in to the admin area.
type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"` // admin | member
	CreatedAt   int64  `json:"createdAt"`
}

// IsAdmin reports whether the user may manage the site.
func (u *User) IsAdmin() bool { return u != nil && u.Role == "admin" }

// Notice is a dashboard announcement.
type Notice struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"` // info | success | warning | danger
	Link      string `json:"link"`
	Active    bool   `json:"active"`
	Sort      int    `json:"sort"`
	ExpiresAt int64  `json:"expiresAt"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// Image is an uploaded asset. Bytes are only loaded when serving the asset.
type Image struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mime      string `json:"mime"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	CreatedAt int64  `json:"createdAt"`
}

// Stats is the admin dashboard summary.
type Stats struct {
	Categories   int     `json:"categories"`
	Cards        int     `json:"cards"`
	Pending      int     `json:"pending"`
	Users        int     `json:"users"`
	Images       int     `json:"images"`
	TotalClicks  int64   `json:"totalClicks"`
	ViewsToday   int64   `json:"viewsToday"`
	TopCards     []Card  `json:"topCards"`
	DailyViews   []Daily `json:"dailyViews"`
	RecentCards  []Card  `json:"recentCards"`
	DatabaseSize int64   `json:"databaseSize"`
}

// Daily is one bucket of the traffic chart.
type Daily struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}
