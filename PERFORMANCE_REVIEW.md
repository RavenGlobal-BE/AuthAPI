# Raven Auth: Performance Review

**Scope:** every Go file in the repo, plus `Dockerfile`, `docker-compose.yml`, `go.mod` and the deploy workflow.
**Target toolchain:** Go 1.27.2. Every API suggested below exists in Go 1.27 (checked against `$GOROOT/api/go1.*.txt`) or in the dependency versions pinned in `go.sum` (go-redis v9.17.3, pgx v5.8.0, gin v1.11.0, golang-jwt v5.3.1).
**No source files were changed.** This document is the only new file.

---

## TL;DR: where the time goes

Most of the latency is not in Postgres. Your SQL queries are already about as cheap as they can be: every lookup uses a primary key or a `UNIQUE` index, and pgx caches prepared statements by default. The slow parts are the work done **around** the database:

| # | Finding | Hits | Expected gain | Effort |
|---|---|---|---|---|
| 1 | **External geo-IP HTTP call, made synchronously and with no timeout** (`GetIPLocation`) | `/login`, `/token`, `/refresh` (rotation) | Removes a full internet round-trip from each request (often 50–300 ms, and unbounded if ip-api hangs) | Low |
| 2 | **SMTP email sent synchronously** inside the request | `/register`, `/requestReset`, `/login` (unverified users) | Removes hundreds of ms to several seconds | Low |
| 3 | **Every JWT is parsed and verified twice** (`ValidateToken` calls `ParseWithClaims` inside its own keyfunc) | Every authenticated route, plus `/refresh`, `/introspect`, `/authorize`, `/logout` | **2.3× faster validation** (measured: 114 µs → 49 µs) | Low |
| 4 | **The client cache is never filled**, so `GetClientByID` always queries Postgres | `/login`, `/token`, `/authorize`, `/clientInfo` | One DB round-trip removed per request | Low |
| 5 | **Argon2id used for `client_secret`** (high-entropy secrets don't need a slow hash) | `/token` (confidential clients) | About 50 ms of CPU and 48 MiB of allocation removed per token exchange | Low/Med |
| 6 | **Argon2id cost per login**: 52 ms and 48 MiB per call, no concurrency cap | `/login`, `/register`, `/reset` | Prevents CPU and memory collapse under load; optionally about 40% faster with OWASP parameters | Medium |
| 7 | **Redis commands sent one at a time** (HSET, then EXPIRE, then SADD…; N+1 in `GetDevices`; 3 calls in the rate limiter) | Almost every route | 2–3 Redis round-trips become 1; atomic single-use codes | Low |
| 8 | **`context.Background()` everywhere**, no timeouts | All DB and Redis calls | Slow dependencies can no longer pile up goroutines and connections | Medium |
| 9 | **pgxpool tuning** (`MaxConns=50` × 2 containers during blue/green, no warm conns, synchronized recycling) | Startup and every 10 min | Fewer latency spikes; avoids running out of `max_connections` | Low |
| 10 | Smaller items: regex compiled per call, debug `Sprintf` executed in production, no HTTP server timeouts, PGO not used | Various | Small but free | Low |

**Order to do them in:** 0 (fix the build and add measurement), then 1, 2, 3, 4, 7, 5, 6, 8, 9, 10.

Separately, the review found **correctness and security bugs**. Some of them crash requests, and two mean logout doesn't actually log anyone out. They're listed in the [Appendix](#appendix-bugs-found-along-the-way). Several of them affect performance too, because a recovered panic is a slow 500.

---

## 0. Before optimizing

### 0.1 The build is currently broken on Go 1.27

```
$ go vet ./...
personalization.go:78:15: uuid.New requires go1.27 or later (file is go1.25)
```

`personalization.go` imports the new standard-library `uuid` package, which was added in Go 1.27. But `go.mod` still says `go 1.25.5`, so the compiler refuses to build it.

**`go.mod`:**
```diff
-go 1.25.5
+go 1.27
```

Use `go 1.27` rather than `go 1.27.2`. The official `golang` Docker images set `GOTOOLCHAIN=local`, so `go 1.27.2` in `go.mod` would make the build fail on any image or machine still at 1.27.1. Your Dockerfile and your local Homebrew Go are both 1.27.1 right now.

**`Dockerfile`:** bump the builder image to match your target:
```diff
-FROM golang:1.27.1-alpine3.24 AS builder
+FROM golang:1.27.2-alpine3.24 AS builder
```
(Check that this exact tag exists on Docker Hub. If it doesn't, `golang:1.27.2-alpine` works.)

Then run `go mod tidy`.

### 0.2 Measure first so you can confirm each win

Add `pprof` on a **loopback-only** port. It runs on the default `http.ServeMux`, separate from gin, so it never reaches the public internet:

```go
// auth.go
import _ "net/http/pprof"

// inside main(), before r.Run / srv.ListenAndServe
go func() {
	_ = http.ListenAndServe("127.0.0.1:6060", nil)
}()
```

```bash
# inside the container (docker exec) or via an SSH tunnel:
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30
```

Also add a timing middleware to see per-route latency in your logs:

```go
func Timing() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logging.Log(fmt.Sprintf("%s %s %d %s", c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start)), logging.Info)
	}
}
// r.Use(gin.Recovery(), Timing())
```

---

## Hot-path map (what each request does today)

| Route | Work per request (in order) |
|---|---|
| `POST /login` | Redis GET, INCR, (EXPIRE) → **PG** client lookup *(cache broken)* → **PG** user by email → **Argon2id ~52 ms** → 2× Ed25519 sign → **HTTP to ip-api.com** → Redis HSET, SADD, EXPIRE |
| `POST /token` | **PG** client lookup → **Argon2id on client_secret ~52 ms** → Redis HGETALL, DEL → **PG** user by id → 2× sign → **HTTP to ip-api.com** → Redis HSET, SADD, EXPIRE |
| `POST /refresh` | JWT validate **×2** → Redis HGETALL → (PG user) → sign → if rotating: sign ×2, DEL, **HTTP to ip-api.com**, HSET, SADD, EXPIRE; else EXPIRE |
| `POST /userinfo` | JWT validate **×2** → Redis HGETALL → **PG** user by id |
| `POST /introspect` | JWT validate **×2** → Redis HGETALL |
| `GET /authorize` | **PG** client lookup → JWT validate ×2 (and again ×2 for the refresh cookie) → Redis HSET, EXPIRE |
| `POST /register` | Redis rate limit → Argon2id → **PG** insert → Redis HSET, EXPIRE → **SMTP send (sync)** |
| `GET /requestReset` | Redis rate limit → **PG** user → Redis HSET, EXPIRE → **SMTP send (sync)** |

---

## 1. Take the geo-IP lookup off the request path (biggest win)

**Where:** `Authorization/authorization.go:91-123`, called from `routes.go:137`, `routes.go:356`, `routes.go:453`.

```go
resp, err := http.Get(url) // http.DefaultClient: NO timeout
```

Problems:
- It's a **full round-trip to a third-party server on the public internet**, made in the middle of `/login`, `/token` and `/refresh`. Clients wait for it.
- `http.DefaultClient` has **no timeout**. If ip-api.com stalls, your login stalls with it, holding a goroutine, a Redis connection and the user's request.
- The free ip-api.com endpoint is HTTP-only, rate-limited (about 45 requests per minute per source IP) and licensed for non-commercial use only. Under real traffic you'll get throttled responses, which currently decode silently into `nil` values.

### Fix A (recommended): do it in the background and patch the session afterwards

The session is written immediately, without location data, and a goroutine fills in the location later. The user never waits for it.

```go
// Authorization/authorization.go
var geoClient = &http.Client{Timeout: 2 * time.Second}

func GetIPLocation(ctx context.Context, ip string) (map[string]any, error) {
	if ip == "::1" || ip == "127.0.0.1" {
		return map[string]any{"country": "US", "city": "San Francisco", "lat": 37.7749, "lon": -122.4194, "carrier": "Localhost"}, nil
	}

	// `fields` makes ip-api return only what you use (smaller payload, faster decode).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://ip-api.com/json/"+ip+"?fields=status,countryCode,regionName,lat,lon,isp", nil)
	if err != nil {
		return nil, err
	}
	resp, err := geoClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geo lookup: %s", resp.Status)
	}

	var data struct {
		Status      string  `json:"status"`
		CountryCode string  `json:"countryCode"`
		RegionName  string  `json:"regionName"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		ISP         string  `json:"isp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("error decoding json: %w", err)
	}
	if data.Status != "success" {
		return nil, errors.New("geo lookup failed")
	}
	return map[string]any{"country": data.CountryCode, "city": data.RegionName, "lat": data.Lat, "lon": data.Lon, "carrier": data.ISP}, nil
}
```

The new Redis helper. It only writes if the session still exists, so a goroutine that runs after logout can't resurrect a session key without a TTL:

```go
// DatabaseEngine/TokenRepo.go
var hsetIfExists = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
  return redis.call('HSET', KEYS[1], unpack(ARGV))
end
return 0`)

func (tr *TokenRepo) SetSessionLocation(ctx context.Context, userID int64, sessionID string, loc map[string]any) error {
	key := "session:" + strconv.FormatInt(userID, 10) + ":" + sessionID
	return hsetIfExists.Run(ctx, tr.redis.client, []string{key},
		"carrier", loc["carrier"], "lat", loc["lat"], "lon", loc["lon"]).Err()
}
```

In the handlers (`handleLogin`, `token`, the rotation branch of `refresh`):

```go
// Pass nil. CreateSessionData only *reads* from the map, and reading a nil map is safe in Go.
sessionData := a.userRedis.CreateSessionData(user.UserID, loginData.ClientID, loginData.DeviceModel, loginData.DeviceName, loginData.Language, false, nil)
a.userRedis.InsertSession(ctx, sessionID, user.UserID, sessionData, 14*24*time.Hour)

// Copy anything you need out of gin.Context BEFORE the goroutine starts:
// gin recycles *gin.Context after the handler returns.
clientIP := c.ClientIP()
userID := user.UserID
bg := context.WithoutCancel(c.Request.Context()) // keeps values, drops the cancel signal of the finished request
go func() {
	ctx, cancel := context.WithTimeout(bg, 3*time.Second)
	defer cancel()
	loc, err := auth.GetIPLocation(ctx, clientIP)
	if err != nil {
		logger.Log(err.Error(), logger.Warning)
		return
	}
	if err := a.userRedis.SetSessionLocation(ctx, userID, sessionID, loc); err != nil {
		logger.Log(err.Error(), logger.Warning)
	}
}()
```

### Fix B (best long-term): a local GeoIP database

Use MaxMind GeoLite2 (free with an account) and `github.com/oschwald/geoip2-golang`. A lookup is a memory-mapped read that takes **microseconds**, with no network, no rate limits and no licensing problem. Load the `.mmdb` once at startup and keep the `*geoip2.Reader` on `App`. Carrier/ISP data needs the separate GeoLite2-ASN database.

---

## 2. Send emails asynchronously

**Where:** `routes.go:93`, `routes.go:508`, `routes.go:639`, which call `Mailer/main.go:57` (`smtp.SendMail`).

`smtp.SendMail` opens a new TCP connection, does STARTTLS and AUTH, and sends the message, all **while the HTTP request waits**. That is typically several hundred ms, and seconds if SES is slow.

It also causes a **user-enumeration timing leak** in `/requestReset`. Existing emails take visibly longer to answer than unknown ones, even though the response body is the same on purpose.

Minimal fix (the arguments are evaluated before the goroutine starts, so this is safe with gin's context recycling):

```go
go a.mailService.AccountVerificationEmail(req.Email, req.FirstName, key, req.Language)
...
go a.mailService.ResetPasswordEmail(mail, user.FirstName, key, language)
```

Durable fix (optional): push `{template, to, params}` onto a Redis list and have a small worker goroutine `BLPOP` and send. That way a container restart during blue/green deploys doesn't drop emails.

---

## 3. `ValidateToken` parses and verifies every token twice

**Where:** `Authorization/jwtManager.go:98-157`.

The keyfunc passed to the outer `jwt.ParseWithClaims` calls `jwt.ParseWithClaims` **again** on the same token. Each validation therefore does:
- 2× base64 + JSON decode of header and claims
- **2× Ed25519 signature verification** (the expensive part)
- a base64 decode of `JWT_PUBLIC_KEY` from the environment, plus several `os.Getenv` calls

The manual `ExpiresAt` check is also redundant, because `WithExpirationRequired()` plus the default validator already enforces `exp`, `nbf` and `iat`.

**Measured on this machine (Apple M2, Go 1.27.1, golang-jwt v5.3.1):**

```
BenchmarkValidateNested-8   114011 ns/op   3472 B/op   71 allocs/op   ← current
BenchmarkValidateSingle-8    48883 ns/op   1659 B/op   34 allocs/op   ← proposed
```

That's a **2.3× speed-up and half the allocations** on every authenticated request.

### Fix: load keys once, build one reusable parser, parse once

```go
// Authorization/jwtManager.go
import "sync"

type jwtKeys struct {
	priv   ed25519.PrivateKey
	pub    ed25519.PublicKey
	kid    string
	issuer string
	parser *jwt.Parser // safe for concurrent use: it only holds options
}

// sync.OnceValues runs the function once and caches (value, error).
var loadKeys = sync.OnceValues(func() (*jwtKeys, error) {
	priv, err := getPrivateKey()
	if err != nil {
		return nil, err
	}
	pub, err := getPublicKey()
	if err != nil {
		return nil, err
	}
	k := &jwtKeys{priv: priv, pub: pub, kid: os.Getenv("JWT_KID"), issuer: os.Getenv("JWT_ISSUER")}
	if k.kid == "" {
		return nil, errors.New("JWT_KID .env variable not set")
	}
	k.parser = jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(k.issuer),
		jwt.WithExpirationRequired(),
	)
	return k, nil
})

// Call from main() right AFTER godotenv.Load(). It fails fast on a bad key, and it
// guarantees the Once never caches an "env not loaded yet" error.
func InitKeys() error {
	_, err := loadKeys()
	return err
}

func ValidateToken(tokenString string) (*JWTPayload, error) {
	k, err := loadKeys()
	if err != nil {
		return nil, err
	}

	claims := &JWTPayload{}
	token, err := k.parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid != k.kid {
			return nil, fmt.Errorf("unknown kid: %q", kid)
		}
		return k.pub, nil
	})
	if err != nil {
		logger.Log(err.Error(), logger.Debug)
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	// len()==0 instead of ==nil: callers index Audience[0], and `"aud": []` would panic.
	if len(claims.Audience) == 0 {
		return nil, errors.New("invalid token audience")
	}
	if claims.TokenType != "access" && claims.TokenType != "refresh" {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}
```

And in `GenerateJWTToken`, use the cached values:

```go
k, err := loadKeys()
if err != nil {
	return "", err
}
...
Issuer:  k.issuer,
Subject: strconv.FormatInt(userID, 10), // instead of fmt.Sprintf("%d", ...)
...
token.Header["kid"] = k.kid
tokenString, err := token.SignedString(k.priv)
```

In `main()`:
```go
if err := godotenv.Load(); err != nil { ... }
if err := auth.InitKeys(); err != nil {
	logging.Log(err.Error(), logging.Fatal)
}
```

> Related: `jwtconfigroutes.go:30` hard-codes `"kid": "raven_ed25519-2026-02-25"` in the JWKS, while tokens are signed with `os.Getenv("JWT_KID")`. If the two ever differ, third-party verifiers will reject your tokens. Serve `k.kid` there instead. You can also build the JWKS and OpenID JSON once at startup, since they never change.

---

## 4. The client cache is never filled

**Where:** `DatabaseEngine/ClientsRepo.go:44-85`.

`GetClientByID` **reads** from `cr.cache` but never **writes** to it, so every call queries Postgres. `Cleanup()` is never called either. This lookup is on `/login`, `/token`, `/authorize` and `/clientInfo`, so it's one wasted round-trip per request.

The comment also says *"Returns nil if not found or inactive"*, but the query doesn't filter on `active`. **Deactivated apps can still log users in.**

### Fix

```go
const clientCacheTTL = 5 * time.Minute

func (cr *ClientsRepo) GetClientByID(ctx context.Context, clientID string) *ClientInfo {
	now := time.Now()

	cr.mu.RLock()
	cached, found := cr.cache[clientID]
	cr.mu.RUnlock()
	if found && now.Before(cached.expiresAt) {
		return cached.data
	}

	var app ClientInfo
	err := cr.db.pool.QueryRow(ctx, `
		SELECT app_id, client_id, client_secret, app_name, company, redirect_uri, is_public, active, created_at, company_icon
		FROM accounts.apps
		WHERE client_id = $1 AND active = true
	`, clientID).Scan(
		&app.AppID, &app.ClientID, &app.ClientSecret, &app.AppName, &app.Company,
		&app.RedirectURI, &app.IsPublic, &app.Active, &app.CreatedAt, &app.CompanyIcon,
	)
	if err != nil {
		return nil
	}

	cr.mu.Lock()
	cr.cache[clientID] = cachedClient{data: &app, expiresAt: now.Add(clientCacheTTL)} // overwrites stale entries; no delete needed
	cr.mu.Unlock()
	return &app
}
```

Notes:
- Only *valid* clients are cached, so the map is bounded by the number of registered apps, and `Cleanup()` isn't needed.
- Trade-off: deactivating an app or changing its redirect URI takes up to `clientCacheTTL` to apply. Lower the TTL, or add an admin "flush" call.
- `/clientInfo` is unauthenticated and not rate-limited, and **misses** still go to Postgres. Put `auth.RateLimiting` on it, or accept the cost.
- Optional: `golang.org/x/sync/singleflight` (already in your module graph) collapses concurrent misses for the same `client_id` into one query when the cache expires.

---

## 5. Don't use Argon2id for `client_secret`

**Where:** `routes.go:281`, `auth.CheckPasswordHash(clientSecret, client.ClientSecret)`.

Argon2id exists to protect **low-entropy, human-chosen** passwords from brute force. Client secrets should be **machine-generated random strings** of at least 32 bytes, and those can't be brute-forced however fast the hash is. Making them slow only costs you about 52 ms of CPU and 48 MiB of RAM on every `/token` call from a confidential client.

Standard practice for high-entropy secrets is SHA-256 plus a constant-time compare:

```go
// Authorization/authorization.go
func CheckClientSecret(secret, storedSHA256Hex string) bool {
	sum := sha256.Sum256([]byte(secret))
	want, err := hex.DecodeString(storedSHA256Hex)
	if err != nil || len(want) != len(sum) {
		return false
	}
	return subtle.ConstantTimeCompare(sum[:], want) == 1
}
```

Migration: when you issue or rotate a secret, generate it with `crypto/rand` (at least 32 bytes) and store `SHA256Hash(secret)`, which you already have in `authorization.go:44`. During the transition, keep the Argon2 path for rows whose value starts with `$argon2id$`.

> ⚠️ Only do this if your client secrets really are randomly generated. If humans have chosen any of them, keep Argon2 for those.

---

## 6. Argon2id on login: cap concurrency, encode parameters, consider tuning

**Where:** `Authorization/authorization.go:20-72`.

**Measured (Apple M2, single call):**
```
BenchmarkArgon2Current-8           52.0 ms/op   50,336,145 B/op   ← t=3, m=48 MiB, p=2
BenchmarkArgon2OWASP_m46_t1_p1-8   31.4 ms/op   48,236,389 B/op   ← OWASP option: t=1, m=46 MiB, p=1
```
Server vCPUs are usually slower per core than an M2, so expect production numbers to be higher.

### 6.1 Cap concurrent hashes (the most important part)

Each hash allocates about **48 MiB**, and nothing limits how many run at once. 50 concurrent logins need about **2.4 GiB**, and the GC and CPU then thrash for everyone, including cheap routes like `/userinfo`. (The README says 256 MB of RAM is enough. That only holds for about 5 concurrent logins.)

```go
// Authorization/authorization.go
// Roughly vCPUs / argonThreads. With 8 vCPU and p=2 → 4. Tune with load tests.
var hashSlots = make(chan struct{}, 4)

func acquireHashSlot() func() {
	hashSlots <- struct{}{}
	return func() { <-hashSlots }
}

func HashPassword(password string) (string, error) {
	defer acquireHashSlot()()
	...
}

func CheckPasswordHash(password, encodedHash string) bool {
	defer acquireHashSlot()()
	...
}
```
Requests now queue briefly instead of all slowing down together, and peak memory is bounded (4 × 48 MiB ≈ 192 MiB).

Also set a memory limit so the GC works with the container instead of against it:
```dockerfile
ENV GOMEMLIMIT=900MiB   # ~90% of the container's memory limit
```

### 6.2 Store the parameters in the hash (do this before changing parameters)

The current format `$argon2id$<salt>$<hash>` doesn't record `m`, `t` or `p`. If you ever change the constants, **every existing password stops verifying**. Switch to the standard PHC format, which tools like `argon2` CLI and libsodium also read:

```go
func HashPassword(password string) (string, error) {
	defer acquireHashSlot()()
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}
```

In `CheckPasswordHash`:
- `len(parts) == 6` means the new format: parse `m,t,p` with `fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)`.
- `len(parts) == 4` means a legacy hash: use the current constants.

Then **rehash on successful login** whenever the stored params differ from the current ones, the same way you already do for bcrypt.

### 6.3 Optional: tune parameters

OWASP's Password Storage Cheat Sheet lists `m=46 MiB, t=1, p=1` as an equivalent-strength option, and it was about 40% faster in the benchmark above. This is a security decision, so only change it after 6.2 is in place.

---

## 7. Batch Redis commands (pipelines and transactions)

Every `HSET` followed by `EXPIRE` is **two network round-trips**, and if the process dies between them the key lives forever. go-redis v9 can send several commands in one round-trip with `Pipelined`, or atomically with `TxPipelined` (`MULTI/EXEC`).

### 7.1 Shared helper for "hash with TTL"

```go
// DatabaseEngine/TokenRepo.go
import "github.com/redis/go-redis/v9"

func (tr *TokenRepo) hsetWithTTL(ctx context.Context, key string, data map[string]any, ttl time.Duration) error {
	_, err := tr.redis.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, key, data)
		p.Expire(ctx, key, ttl)
		return nil
	})
	return err
}
```
Use it in `InsertToken`, `InsertAuthCode`, `SignUpInsert`, `ResetInsert` and `InsertOPToken`. Each one shrinks to a single call.

### 7.2 `InsertSession`: 3 round-trips → 1, and the per-user index no longer grows forever

```go
func (tr *TokenRepo) InsertSession(ctx context.Context, sessionID string, userID int64, data map[string]any, ttl time.Duration) error {
	uid := strconv.FormatInt(userID, 10)
	sessionKey := "session:" + uid + ":" + sessionID
	setKey := "user:sessions:" + uid

	_, err := tr.redis.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, sessionKey, data)
		p.Expire(ctx, sessionKey, ttl)
		p.SAdd(ctx, setKey, sessionID)
		p.Expire(ctx, setKey, ttl) // currently the set never expires and only grows
		return nil
	})
	return err
}
```
It now returns an `error`. Today all three results are silently discarded, so a failed session write still gives the user a "successful" login with tokens that won't work.

### 7.3 Single-use codes: atomic read-and-delete

`GetAuthCode`, `VerifyReset` and `VerifySignup` run `HGETALL` and then `DEL` as two separate commands. Besides costing two round-trips, this is a **race**: two concurrent requests can both read the code before either deletes it, which defeats "single use". OAuth requires auth codes to be single-use.

```go
func (tr *TokenRepo) consumeHash(ctx context.Context, key string) (map[string]string, error) {
	var get *redis.MapStringStringCmd
	_, err := tr.redis.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		get = p.HGetAll(ctx, key)
		p.Del(ctx, key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(get.Val()) == 0 {
		return nil, errors.New("not found or expired")
	}
	return get.Val(), nil
}

func (tr *TokenRepo) GetAuthCode(ctx context.Context, code string) (map[string]string, error) {
	return tr.consumeHash(ctx, "auth_code:"+code)
}
// same for VerifyReset ("reset:"+link) and VerifySignup ("signup:"+link)
```

### 7.4 `GetDevices`: N+1 round-trips → 2, with stale cleanup

```go
func (tr *TokenRepo) GetDevices(ctx context.Context, userID int64) ([]map[string]string, error) {
	uid := strconv.FormatInt(userID, 10)
	setKey := "user:sessions:" + uid

	ids, err := tr.redis.client.SMembers(ctx, setKey).Result()
	if err != nil || len(ids) == 0 {
		return nil, err
	}

	cmds := make([]*redis.MapStringStringCmd, len(ids))
	if _, err := tr.redis.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for i, id := range ids {
			cmds[i] = p.HGetAll(ctx, "session:"+uid+":"+id)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	devices := make([]map[string]string, 0, len(ids))
	var stale []any
	for i, cmd := range cmds {
		if m := cmd.Val(); len(m) > 0 {
			devices = append(devices, m)
		} else {
			stale = append(stale, ids[i]) // session expired, drop it from the index
		}
	}
	if len(stale) > 0 {
		tr.redis.client.SRem(ctx, setKey, stale...)
	}
	return devices, nil
}
```
The parameter changes to `int64`, which also fixes the `/devices` panic (see Appendix #6). In the handler: `userID.(int64)`.

### 7.5 Rate limiter: 3 round-trips → 1

Today it's `GET` then `INCR` then `EXPIRE` (the last only on the first hit), and the error check is a string comparison (`err.Error() == "redis: nil"`).

```go
// DatabaseEngine/RatelimitRepo.go
// Requires Redis >= 7.0 for EXPIRE ... NX
func (rr *RateRepo) Hit(ctx context.Context, address string, window time.Duration) (int64, error) {
	key := "ratelimit:" + address
	var incr *redis.IntCmd
	_, err := rr.redis.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		incr = p.Incr(ctx, key)
		p.ExpireNX(ctx, key, window) // only sets a TTL if none exists, so the window isn't extended
		return nil
	})
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
```

```go
// Authorization/middleware.go
const limit = 15
count, err := rr.Hit(c.Request.Context(), clientIP, 15*time.Minute)
if err != nil || count > limit {
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
	return
}
c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
c.Header("X-RateLimit-Remaining", strconv.FormatInt(max(0, limit-count), 10))
c.Next()
```
This allows the same 15 requests per window as today. If your Redis is older than 7.0, keep `Expire` conditional on `count == 1` (two round-trips) or use a small Lua script.

Anywhere you keep a plain `Get`, replace `err.Error() == "redis: nil"` with `errors.Is(err, redis.Nil)`.

### 7.6 Redis client options

Set timeouts explicitly. The defaults are fine for most things, but it's better to state them:

```go
client := redis.NewClient(&redis.Options{
	Addr:         host,
	Password:     password,
	DB:           database,
	DialTimeout:  2 * time.Second,
	ReadTimeout:  500 * time.Millisecond,
	WriteTimeout: 500 * time.Millisecond,
	MinIdleConns: 4, // keep warm connections so the first requests after idle don't pay the dial cost
})
```

---

## 8. Use the request context, with a timeout

Every repo method uses `context.Background()`. If a client disconnects, or Postgres or Redis is slow, the work continues with no deadline. Under load, those requests pile up and use up the pool (`MaxConns`), and **every** route then starts timing out.

**Step 1:** a middleware that puts a deadline on every request:

```go
func RequestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
// r.Use(gin.Recovery(), RequestTimeout(5*time.Second))
```

**Step 2:** add `ctx context.Context` as the first parameter of every `Usersrepo` method (`GetAccountByEmail`, `GetAccountById`, `ResetPassword`, `RegisterAccount`, `VerifyAccount`, `SetCountryCode`), and pass `c.Request.Context()` from the handlers instead of `context.Background()`:

```go
func (Ur *Usersrepo) GetAccountByEmail(ctx context.Context, mail string) *UserAuth {
	var v UserAuth
	err := Ur.db.pool.QueryRow(ctx, `select ... where email = $1`, mail).Scan(...)
	...
}

// handler
user := a.ur.GetAccountByEmail(c.Request.Context(), loginData.Email)
```

Use `c.Request.Context()`, not `c` itself. `*gin.Context` implements `context.Context`, but unless you enable `engine.ContextWithFallback` it doesn't forward the request's cancellation.

For goroutines that outlive the request (sections 1 and 2), use `context.WithoutCancel(c.Request.Context())` plus their own `WithTimeout`, as shown in section 1.

---

## 9. pgxpool configuration

**Where:** `DatabaseEngine/db.go:16-31`.

```go
config, err := pgxpool.ParseConfig(connString)
if err != nil {
	logger.Log("Unable to parse the PG config", logger.Error) // ← continues with config == nil → panics on the next line
}
config.MaxConns = 50
config.MaxConnLifetime = 10 * time.Minute
```

Problems:
1. **The parse error is ignored**, so a bad `DATABASE_URL` crashes with a nil-pointer panic instead of a clear error.
2. **`MaxConns = 50` per container.** During a blue/green deploy, both containers run at once, which means up to **100 connections**, exactly Postgres's default `max_connections`. More connections than about 2–4× the DB server's cores rarely helps throughput anyway.
3. **No `MinConns`.** After startup or an idle period, the first requests pay the TCP, TLS and auth handshake.
4. **No lifetime jitter.** Connections opened together expire together, so every 10 minutes there's a burst of reconnects and a small latency spike.

```go
func NewDB(ctx context.Context, connString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	config.MaxConns = 20                          // tune: ~2–4× Postgres server cores, minus headroom for blue/green
	config.MinConns = 4                           // warm connections ready at all times
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnLifetimeJitter = 5 * time.Minute // spread out reconnects
	config.MaxConnIdleTime = 5 * time.Minute
	config.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}
```

You can also set these in the DSN (`?pool_max_conns=20&pool_min_conns=4`), so ops can change them without a rebuild.

**Prepared statements:** pgx v5's default `QueryExecModeCacheStatement` already prepares and caches each query per connection, so you get prepared-statement speed for free. If you ever put **PgBouncer in transaction mode** in front of Postgres, switch to `config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe` (or `Exec`), or queries will fail.

**Indexes:** no changes needed. `user_id` is the primary key, and `email` and `client_id` are `UNIQUE`, so each already has a B-tree index.

### 9.1 Optional: avoid the DB on `/userinfo`

`/userinfo` does a Redis `HGETALL` and then a Postgres query, one after the other. The two don't depend on each other: the user ID comes from the token, and `sessionData["user_id"]` is the same value. You can run them in parallel with `sync.WaitGroup.Go` (Go 1.25+):

```go
ctx := c.Request.Context()
uid := userID.(int64)

var (
	wg      sync.WaitGroup
	sessErr error
	user    *dbEngine.UserAuth
)
wg.Go(func() { _, sessErr = a.userRedis.GetSessionByID(ctx, redisKey) })
wg.Go(func() { user = a.ur.GetAccountById(ctx, uid) })
wg.Wait()

if sessErr != nil { c.JSON(404, gin.H{"error": "Session not found"}); return }
if user == nil   { c.JSON(404, gin.H{"error": "User not found"}); return }
```

Also consider a profile query that doesn't `SELECT password`. `/userinfo` and `/refresh` never need the hash, so there's no reason to send about 100 bytes of secret over the wire on every call.

A bigger step: cache the profile in Redis (`profile:<uid>`, TTL a few minutes), and delete it in `UpdateUserSettings`, `UpdatePhoto`, `RemovePhoto` and `SetCountryCode`.

---

## 10. Smaller items

### 10.1 `EmailIsValid` compiles a regex on every call (and can panic)
`Mailer/mailverification.go:19`, `regexp.MatchString` compiles the pattern every time. It also reads `addr.Address` **before** checking `err`, so any unparsable email (`"foo"`) causes a nil-pointer panic and a 500.

```go
var hasLowercase = regexp.MustCompile(`[a-z]`)

func EmailIsValid(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !hasLowercase.MatchString(addr.Address) {
		return false
	}
	user, domain, _ := strings.Cut(strings.ToLower(addr.Address), "@")
	...
}
```

### 10.2 Debug logging costs CPU in production
`Authorization/middleware.go:23` runs `fmt.Sprintf("Auth Header: %s, Cookie: %s", ...)` on **every authenticated request**, even when debug logging is off. `Log()` throws the string away afterwards, but the formatting has already happened. It also **writes bearer tokens to the logs** in development. `displayConfigs()` logs the DB URL, Redis password and mail password.

Read the environment once and guard expensive debug lines:

```go
// Logging/main.go
var debugEnabled atomic.Bool

// Call once in main() after godotenv.Load().
func Init() { debugEnabled.Store(os.Getenv("ENVIRONMENT") == "DEVELOPMENT") }

func DebugEnabled() bool { return debugEnabled.Load() }

func Log(message string, logType LogType) {
	if logType == Debug && !debugEnabled.Load() {
		return
	}
	...
}
```
```go
// middleware
if logger.DebugEnabled() {
	logger.Log(fmt.Sprintf("auth: header=%t cookie=%t", authHeader != "", cookie != ""), logger.Debug) // don't log the token itself
}
```
Longer term, `log/slog` with a JSON handler gives you levels, structured fields and cheap disabled logging out of the box.

### 10.3 HTTP server timeouts
`r.Run()` uses an `http.Server` with **no timeouts**, so slow or stuck clients hold goroutines and file descriptors indefinitely:

```go
srv := &http.Server{
	Addr:              fmt.Sprintf(":%d", port),
	Handler:           r,
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       15 * time.Second, // enough for an 8 MiB upload
	WriteTimeout:      15 * time.Second,
	IdleTimeout:       120 * time.Second,
}
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
	logging.Log(err.Error(), logging.Fatal)
}
```
For blue/green, also handle `SIGTERM` with `srv.Shutdown(ctx)` so in-flight logins finish before the old container is removed (`signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)`).

### 10.4 Profile-Guided Optimization (free 2–14% CPU)
Since Go 1.21, `go build` automatically uses a `default.pgo` file in the main package directory. Capture a CPU profile from production under real traffic, using the pprof endpoint from section 0.2:

```bash
curl -o default.pgo "http://127.0.0.1:6060/debug/pprof/profile?seconds=60"
```
Commit `default.pgo` at the repo root, next to `auth.go`. No Dockerfile change is needed (`-pgo=auto` is the default). The Go team reports typical gains of 2–14%. Refresh the profile every few releases.

### 10.5 Dockerfile
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o auth .
...
FROM alpine:3.24        # pin instead of :latest for reproducible deploys
ENV GOMEMLIMIT=900MiB   # see 6.1, set to ~90% of the container memory limit
```
`-s -w` strips the symbol tables, so the binary is smaller and pulls faster during deploys (runtime speed is unaffected).

### 10.6 Snowflake node created on every registration
`DatabaseEngine/UsersRepo.go:89-95` sets the global `snowflake.Epoch` and builds a new `Node` on every call. That's wasteful, and it's also a **correctness bug**: each new node restarts its sequence at 0, so two registrations in the same millisecond can produce the **same `user_id`**, and the second one fails with a primary-key violation.

```go
var idNode = sync.OnceValues(func() (*snowflake.Node, error) {
	snowflake.Epoch = 1748736000000
	return snowflake.NewNode(1) // during blue/green both containers are node 1; consider a NODE_ID env var
})

// in RegisterAccount
node, err := idNode()
if err != nil {
	return err
}
userid := node.Generate().Int64()
```

### 10.7 Things you get for free on Go 1.27 (no action needed)
- **JSON:** `GOEXPERIMENT=jsonv2` is on by default in 1.27 (see `internal/buildcfg/exp.go`), so `encoding/json`, which gin uses by default, now runs on the faster v2 engine.
- **Green Tea GC** is the default, which helps with the Argon2 allocation churn.
- **Container-aware `GOMAXPROCS`** (since 1.25). The runtime respects the container's CPU limit, so you don't need `automaxprocs`.
- **Benchmarks:** use `for b.Loop() { ... }` (1.24+) when you add benchmarks for the hashing and JWT code.

---

## Benchmark used for the numbers above

Run on an Apple M2 with Go 1.27.1, golang-jwt v5.3.1 and x/crypto v0.46.0, from a scratch module outside the repo. To reproduce, put it in `Authorization/bench_test.go`, adapting it to call your real `ValidateToken` and `CheckPasswordHash`:

```go
func BenchmarkValidateToken(b *testing.B) {
	tok, _ := GenerateJWTToken(1, "a@b.c", "A", "B", "access", time.Now().Add(time.Hour), "n", "sid", "BE", "aud")
	for b.Loop() {
		if _, err := ValidateToken(tok); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckPasswordHash(b *testing.B) {
	h, _ := HashPassword("correct horse battery staple")
	for b.Loop() {
		CheckPasswordHash("correct horse battery staple", h)
	}
}
```
```bash
JWT_PRIVATE_KEY=... JWT_PUBLIC_KEY=... JWT_KID=test JWT_ISSUER=test \
  go test ./Authorization -bench . -benchmem
```

---

## Appendix: bugs found along the way

These aren't performance issues, but several crash requests (gin's `Recovery` turns the panic into a 500) or weaken security. They're listed by severity.

| # | Severity | Where | Problem | Fix |
|---|---|---|---|---|
| 1 | 🔴 Build | `go.mod` | `go 1.25.5` + stdlib `uuid` means **the project doesn't compile** | Section 0.1 |
| 2 | 🔴 Security | `routes.go:533`, `routes.go:451` → `TokenRepo.DeleteToken` | Deletes key `<sessionID>`, but sessions are stored at `session:<uid>:<sessionID>`. **Logout doesn't revoke the session**, and refresh-token rotation leaves the old refresh token valid. | `Del(ctx, "session:"+uid+":"+sid)` and `SRem` from `user:sessions:<uid>` |
| 3 | 🔴 Security | `ClientsRepo.go:61-65` | Query doesn't filter `active`, so **deactivated apps keep working** | Section 4 |
| 4 | 🔴 Security | `TokenRepo.go:146-161` (+ reset/signup) | Read-then-delete isn't atomic, so auth codes and reset links can be **used twice** under concurrency | Section 7.3 |
| 5 | 🟠 Crash | `routes.go:307-308`, `routes.go:345,350` | `*user.CountryCode` is dereferenced even when it's `nil`. The code just above explicitly handles the `nil` case for the `setup` flow, and then panics. | Use a local `country := ""` and set it if non-nil, like `lastName` |
| 6 | 🟠 Crash | `routes.go:907` (`/devices`), `routes.go:842` (`/countryCode`) | `userID.(int)`, but the middleware stores `int64`. `/devices` **panics**; `/countryCode` **always** returns "Invalid token". (Same class as your latest commit's fix on `/userinfo`.) | `userID.(int64)` |
| 7 | 🟠 Crash | `routes.go:423` | `/refresh` with `update_info: true` dereferences `*updatedInfo.LastName`, which is nullable | nil-check first |
| 8 | 🟠 Crash | `Mailer/mailverification.go:18-19` | `addr.Address` read before checking `err`, so an invalid email panics on `/login` and `/requestReset` | Section 10.1 |
| 9 | 🟠 Crash | `personalization.go:95` | `&user.ProfilePicture != nil` is **always true** (it checks the address of the field), so `*user.ProfilePicture` panics for users without a photo. `user` itself can also be nil. | `if user != nil && user.ProfilePicture != nil` |
| 10 | 🟠 Crash | `SecureInput/imageSignatures.go:86-88` | `RemoveImageFromCDN` ignores both errors (flagged by `go vet`), so a nil `res` panics. It also uses `http.DefaultClient` (no timeout) instead of the package's `client`. | Check errors, use `client` |
| 11 | 🟠 Crash | `db.go:19-23` | `ParseConfig` error ignored → nil deref | Section 9 |
| 12 | 🟡 Logic | `routes.go:766` (`/authorize`) | Refresh-cookie fallback looks up `sha256(refreshToken)` via `GetTokenInfo`, but nothing writes that key any more (`InsertToken` is unused). The fallback **always fails** and sends users back to login. | Look up `session:<sub>:<sid>` via `GetSessionByID` |
| 13 | 🟡 Logic | `routes.go:106-115` | Bcrypt migration branch rehashes, then `return`s **without writing a response or issuing tokens**. Users on old hashes must log in twice. `HashPassword`'s error is also ignored. | Rehash, then fall through to token issuance |
| 14 | 🟡 Logic | `routes.go:880-883` (`/update`) | When `middleName` isn't in the body, it's **set to NULL** anyway, so updating only the first name wipes the middle name. Also, an error response is followed by a second `c.JSON(200, …)` (missing `return`). | Only add clauses for non-nil fields; add `return` |
| 15 | 🟡 Logic | `personalization.go:24-26,102-104,110-119` | Missing `return` after error responses, so handlers continue and write a second response or panic on a nil `userID` | Add `return` |
| 16 | 🟡 Logic | `UsersRepo.go:89-95` | Snowflake node recreated per call, so duplicate IDs are possible | Section 10.6 |
| 17 | 🟡 Ops | `TokenRepo.go:338-343` | `user:sessions:<uid>` set never expires or shrinks; `InsertSession` ignores all errors | Section 7.2 |
| 18 | 🟡 Ops | `jwtconfigroutes.go:30` | JWKS `kid` is hard-coded while signing uses `JWT_KID`; they can drift apart | Serve the same value |
| 19 | 🔵 Hygiene | `auth.go:151-157`, `middleware.go:23` | Secrets and bearer tokens are written to logs in DEVELOPMENT | Section 10.2 |
| 20 | 🔵 Hygiene | `routes.go:731,830` | `fmt.Println("Redirecting to: " + redirect)` prints auth codes to stdout in **every** environment | Remove, or log at Debug without the code |
