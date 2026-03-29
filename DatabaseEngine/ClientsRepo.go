package databaseengine

import (
	"context"
	"sync"
	"time"
)

type ClientsRepo struct {
	db    *DB
	cache map[string]cachedClient

	/* This is an rw mutual exclusion lock where we can protect shared data.
	Example: If one person is in the bathroom, others have to wait t'ill the person is done. */
	mu sync.RWMutex
}

type cachedClient struct {
	data      *ClientInfo
	expiresAt time.Time
}

type ClientInfo struct {
	AppID        int32
	ClientID     string
	ClientSecret string // stored as Argon2 hash
	AppName      string
	Company      string
	RedirectURI  string
	IsPublic     bool // true = mobile/SPA (PKCE required), false = server-side (client_secret required)
	Active       bool
	CreatedAt    time.Time
	CompanyIcon  *string // nullable in DB
}

func NewClientsRepo(db *DB) *ClientsRepo {
	return &ClientsRepo{
		db:    db,
		cache: make(map[string]cachedClient),
	}
}

// Looks up a client app by client_id. Returns nil if not found or inactive.
func (cr *ClientsRepo) GetClientByID(ctx context.Context, clientID string) *ClientInfo {
	cr.mu.RLock() //Read-Lock
	cached, found := cr.cache[clientID]
	cr.mu.RUnlock() //Read-Unlock

	if found && time.Now().Before(cached.expiresAt) {
		return cached.data
	}

	if found && time.Now().After(cached.expiresAt) {
		cr.mu.Lock() //Write-Lock
		delete(cr.cache, clientID)
		cr.mu.Unlock() //Write-Unlock
	}

	var app ClientInfo

	row := cr.db.pool.QueryRow(ctx, `
		SELECT app_id, client_id, client_secret, app_name, company, redirect_uri, is_public, active, created_at, company_icon
		FROM accounts.apps
		WHERE client_id = $1
	`, clientID)

	err := row.Scan(
		&app.AppID,
		&app.ClientID,
		&app.ClientSecret,
		&app.AppName,
		&app.Company,
		&app.RedirectURI,
		&app.IsPublic,
		&app.Active,
		&app.CreatedAt,
		&app.CompanyIcon,
	)

	if err != nil {
		return nil
	}

	return &app
}

// Cleanup
func (cr *ClientsRepo) Cleanup() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for clientID, cached := range cr.cache {
		if time.Now().After(cached.expiresAt) {
			delete(cr.cache, clientID)
		}
	}
}
