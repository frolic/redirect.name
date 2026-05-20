# Developing

## Architecture

Go server, no framework. stdlib + `golang.org/x/crypto/acme/autocert` + `golang.org/x/net/publicsuffix`.

- `server.go` — HTTP handler, autocert setup, rate limiting
- `parse.go` — TXT record parsing (regex)
- `translate.go` — URL/path matching and wildcard expansion
- `blocklist.go` — domain blocklist (parsed from `BLOCKLIST` env var)

## Running locally

```bash
# Plain HTTP on :8081
go run .

# With blocklist
BLOCKLIST=evil.com,bad.org go run .

# Custom port
PORT=3000 go run .
```

HTTPS mode activates when `CERT_DIR` is set (autocert/Let's Encrypt).

## Tests

```bash
go test ./...              # unit + integration (no network)
go test -tags e2e -v ./... # live server smoke tests (needs deploy first)
```

## Deploy

Push to `main` triggers the GitHub Actions workflow: test → build → deploy to droplet → smoke test.

```bash
git push origin main
gh run watch  # optional, watch it roll out
```

Manual trigger (no code change needed):

```bash
gh workflow run deploy
```

## Blocklist

Blocks apex domains from all processing — no TXT lookup, no cert issuance, no redirect. Returns 403.

The blocklist is stored as a GitHub repo secret (`BLOCKLIST`), deployed as a systemd `EnvironmentFile`. The local working copy is `blocklist.txt` (gitignored).

### Add domains

Edit `blocklist.txt` (one domain per line), then:

```bash
gh secret set BLOCKLIST < blocklist.txt
gh workflow run deploy
```

### Verify

```bash
# Should return 403
curl -s -o /dev/null -w "%{http_code}" -H "Host: sub.blocked-domain.com" http://45.55.126.223/

# Should still redirect
curl -s -o /dev/null -w "%{http_code}" -H "Host: github.redirect.name" http://45.55.126.223/
```

### How it works

- `blocklist.txt` → `gh secret set` → GitHub secret (newlines preserved)
- Deploy workflow flattens newlines to commas → writes `blocklist.env`
- `blocklist.env` is scp'd to `/usr/local/etc/redirect-name/blocklist.env`
- systemd reads it via `EnvironmentFile=-` (the `-` means don't fail if missing)
- `main()` calls `parseBlocklist(os.Getenv("BLOCKLIST"))` at startup
- `isBlocked()` extracts the apex domain via `publicsuffix.EffectiveTLDPlusOne` and checks the map
- Checked in `redirectHandler` (before TXT lookup) and `hostPolicy` (before cert issuance)

## Infrastructure

- **Server**: `45.55.126.223` (DigitalOcean droplet)
- **DNS alias**: `alias.redirect.name` (CNAME target for users)
- **Docs**: `docs/` directory → GitHub Pages at `redirect.name`
- **Certs**: autocert (Let's Encrypt), stored in `/mnt/certs`, rate limited to 2/apex/week
- **Service**: `redirect-name.service` (systemd), runs as `redirect` user
