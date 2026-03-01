package databaseengine

import (
	"context"
	"time"
)

type ClientsRepo struct {
	db *DB
}

type App struct {
	AppID        int32
	ClientID     string
	ClientSecret string // stored as Argon2 hash
	AppName      string
	Company      string
	RedirectURI  string
	IsPublic     bool // true = mobile/SPA (PKCE required), false = server-side (client_secret required)
	Active       bool
	CreatedAt    time.Time
}

func NewClientsRepo(db *DB) *ClientsRepo {
	return &ClientsRepo{db: db}
}

// Looks up a client app by client_id. Returns nil if not found or inactive.
func (cr *ClientsRepo) GetClientByID(ctx context.Context, clientID string) *App {
	var app App

	row := cr.db.pool.QueryRow(ctx, `
		SELECT app_id, client_id, client_secret, app_name, company, redirect_uri, is_public, active, created_at
		FROM accounts.apps
		WHERE client_id = $1 AND active = true
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
	)

	if err != nil {
		return nil
	}

	return &app
}
