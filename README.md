# render

One Go service for small projects on Render. The home page lives at `/`; Formula War lives at `/formula-war/`.

## Run locally

Requires Go 1.23 or newer. No external Go dependencies or frontend build tools are required.

```sh
go run .
```

Open http://localhost:8080. Set `PORT` to use another port.

```sh
go test -race ./...
go build -trimpath -o bin/server .
```

The server embeds all HTML, CSS, and browser JavaScript into one binary. Formula War's existing browser game and Go multiplayer server are included in `internal/formulawar`; the original source repo is left untouched. Its game documentation is in [internal/formulawar/README.md](internal/formulawar/README.md).

## Render

Create a Render Blueprint from this repository using `render.yaml`, or create a Go web service with:

- Build command: `go build -trimpath -o bin/server .`
- Start command: `./bin/server`
- Health check: `/healthz`

Render supplies `PORT`. No database or secrets are needed. Formula War rooms are held in memory and disappear on restart or redeploy. Keep this service at one instance so all players share the same rooms; free services may sleep when idle.

## Add another project

Put each Go app in `internal/<name>` and expose an `http.Handler`. Register its path in `handler()` in `main.go`, with `http.StripPrefix` when the app expects paths relative to its own root. Keep its browser assets embedded with the app, and ensure browser requests and links use the app's public prefix. Add a project entry to `web/index.html`.

Each app owns its routes and state; the root service owns the process, port, health check, and shutdown.
