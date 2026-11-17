# Sidequests

Small games and tools in one Go service, deployable anywhere that runs Go.

- `/` — project home
- `/formula-war/` — Formula War
- `/undercover/` — 谁是卧底: join a room, configure undercover/blank counts, and privately view cards. Discussion and voting happen offline. Rooms expire after 15 minutes without a room request.
- `/abracada/` — 出包魔法师，中文在线魔法推理桌游，2–5 人标准玩法、六人扩展，自绘 SVG、SSE、可添加机器人。
- `/secret-hitler/` — 揭秘希特勒，1–10 人中文在线桌游，可添加机器人补足规则所需的 5–10 席

## Local development

Requires Go 1.23+. No external Go dependencies or frontend build tools.

```sh
go run .
```

Open http://localhost:8080. Set `PORT` to change the port.

```sh
go test -race ./...
go vet ./...
node --test internal/*/web/*.test.mjs
go build -trimpath -o bin/server .
```

Node is only needed for browser-code tests. The Go binary embeds the frontend and word bank. Game artwork is cached for 24 hours with ETag validation; HTML, CSS, and JavaScript revalidate on reload, and room/private data is never cached. Rooms live in memory; run one instance. Restarting or redeploying clears them. Players authenticate per room with a username and four-digit PIN. The frontend randomly generates and remembers the PIN, and supports changing it in the room. The backend keeps only room credentials in memory and deletes them after 15 minutes without authenticated activity; there is no permanent account registry.

## Deployment

Build with `go build -trimpath -o bin/server .` and run `./bin/server`. The server listens on `PORT` (default `8080`) and exposes `/healthz`. Go buildpacks using `go install .` produce a `sidequests` executable.

### Render

Use `render.yaml`, or configure a Go web service with:

- Build: `go build -trimpath -o bin/server .`
- Start: `./bin/server`
- Health check: `/healthz`
- Root directory: leave empty

For version-triggered releases, set Render's **Auto-Deploy → Off**. Save its **Settings → Deploy Hook** URL as a GitHub Actions secret:

```sh
gh secret set RENDER_DEPLOY_HOOK_URL --repo zpatronus/sidequests
```

Change `DEPLOY_VERSION`, commit, and push to `main`. The workflow runs tests and a build, then requests deployment of that exact commit. Check rollout status in Render.

## Adding an app

Put its Go handler and embedded assets in `internal/<name>`, mount it in `main.go`, and add a card to `web/index.html`. Browser links and requests must use the app's public prefix.

## 谁是卧底 word bank

`internal/undercover/words.tsv` is the only word-bank file. Existing pairs were collected from:

- [imposter-game](https://imposter-game.com/zh/words)
- [心有灵犀](https://ishindenshin.app/zh/guide/undercover-word-list/)
- [巴哈姆特](https://home.gamer.com.tw/artwork.php?sn=6065362)
- [Tooldeer](https://tooldeer.com/undercover.html)

Pairs are converted to Simplified Chinese and deduplicated regardless of order. Celebrity, adult/sexual, crude, and malformed entries are excluded; ordinary body topics such as 狐臭／香港脚 are allowed. No pairs are generated or combined from unrelated words.

## Secret Hitler credits and license

The Chinese adaptation in `internal/secrethitler/` is licensed under **CC BY-NC-SA 4.0**, including its independently written Go game engine and Chinese interface/rules. See [the subdirectory license](internal/secrethitler/LICENSE). This scope does not relicense the other projects.

- **Original game:** Secret Hitler, by Mike Boxleiter, Tommy Maranges, and Mac Schubert / Goat, Wolf & Cabbage. [Official site](https://www.secrethitler.com/) · [Official rules](https://www.secrethitler.com/assets/Secret_Hitler_Rules.pdf).
- **Artwork:** ShrimpCryptid and contributors, [Secret-Hitler-Online](https://github.com/ShrimpCryptid/Secret-Hitler-Online), revision `6b210bae0ae3c3aeb67391dcfa328135a13fb345`, adapted from the original board game. The game uses the original board tracks, role and party cards, policy cards, ballots, player artwork, and victory artwork, plus all 20 avatar SVGs.
- **Changes:** independently implemented authoritative Go rules/state and private views, a new Chinese interface and rules translation, all avatars freely available, optional bots with private-information-only decisions, and no social-sharing unlocks. Upstream game logic was not ported.
- **Terms:** [Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International](https://creativecommons.org/licenses/by-nc-sa/4.0/). Noncommercial use only; adaptations must retain attribution and use the same license. This is an unofficial fan adaptation, not endorsed by the original creators.

## 出包魔法师

`/abracada/` implements the Chinese game with authenticated, per-player SSE views. A player's own spell stones remain hidden during play and are revealed at round end; captured 秘法石板 are fetched separately through the private endpoint. Bots use the same public view and their own captured tablets, with 4 seconds between operations so spell announcements and results remain visible.

- Standard 2–5 player supply: **1, 2, 3, 4, 5, 6, 7, 8** (36 stones).
- Six-player supply: **1, 3, 4, 5, 6, 7, 8, 9** (43 stones). Spell IDs remain 1–8; five secret tablets leave eight stones in the draw pile after dealing.
- Seat order and the first starter are randomized once. Turns proceed counterclockwise; subsequent rounds start with the player who ended the previous round.
- Original game: **Abracada...What?**, designed by Gary Kim. [Chinese rules reference](https://gameurlife.pixnet.net/blog/posts/10335625152) · [English rules reference](https://www.rulespal.com/abracada-what/rulebook).
- Rules, UI and SVG illustrations are independently implemented. The counterclockwise sequence, subsequent-round starter and six-player supply follow this project's selected rules.
