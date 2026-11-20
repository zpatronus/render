package undercover

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

func TestVotingKeepsCardsPrivateAndPreservesHostControl(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/undercover/", http.StripPrefix("/undercover", Handler()))
	server := httptest.NewServer(mux)
	defer server.Close()

	players := map[string]browser{}
	code := ""
	for _, name := range []string{"房主", "甲", "乙", "丙"} {
		jar, _ := cookiejar.New(nil)
		b := browser{t, &http.Client{Jar: jar}, server.URL}
		players[name] = b
		if code == "" {
			code = b.request("POST", "create", map[string]any{"name": name}, 201)["code"].(string)
		} else {
			b.request("POST", "join", map[string]any{"code": code, "name": name}, 200)
		}
	}
	host := players["房主"]
	host.request("POST", "deal", map[string]any{"code": code, "spies": 1, "blanks": 1}, 200)
	post := func(name, path string, ballot, status int, target string) map[string]any {
		t.Helper()
		return players[name].request("POST", path, map[string]any{
			"code": code, "round": 1, "ballot": ballot, "target": target,
		}, status)
	}
	getView := func(name string) roomView {
		t.Helper()
		raw := players[name].request("GET", "room?code="+code, nil, 200)
		data, _ := json.Marshal(raw)
		var result roomView
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	post("甲", "start-vote", 0, 403, "")
	post("房主", "start-vote", 0, 200, "")
	post("房主", "start-vote", 1, 409, "")
	post("房主", "vote", 1, 400, "房主")
	post("房主", "vote", 1, 200, "甲")
	post("房主", "vote", 1, 409, "丙")
	if own := getView("房主").Vote; own.Choice != "甲" || own.Cast != 1 {
		t.Fatalf("original vote was not locked: %+v", own)
	}
	other := getView("乙")
	if other.Vote.Choice != "" || other.Vote.Result != nil {
		t.Fatalf("another player's ballot was exposed: %+v", other.Vote)
	}
	for _, p := range other.Players {
		if p.Role != "" {
			t.Fatal("active player's role was exposed")
		}
	}

	post("甲", "vote", 1, 200, "房主")
	post("乙", "vote", 1, 200, "房主")
	last := post("丙", "vote", 1, 200, "乙")
	result := last["vote"].(map[string]any)["result"].(map[string]any)
	role := result["role"]
	if result["kind"] != "eliminated" || result["name"] != "房主" ||
		(role != "普通人" && role != "卧底" && role != "白板") || len(result) != 3 {
		t.Fatalf("expected only the eliminated host's name and role: %v", result)
	}
	if v := getView("房主"); !v.Host || !v.Vote.CanStart || v.Players[0].Role != role {
		t.Fatalf("eliminated host lost control or role was not revealed: %+v", v)
	}

	post("房主", "start-vote", 1, 200, "")
	post("房主", "vote", 2, 403, "甲")
	post("甲", "vote", 2, 400, "房主")
	post("甲", "vote", 1, 409, "乙")
	post("甲", "vote", 2, 200, "乙")
	post("乙", "vote", 2, 200, "丙")
	post("丙", "vote", 2, 200, "甲")
	if v := getView("房主"); v.Vote.Result.Kind != "tie" || v.Vote.Open || !v.Vote.CanStart || v.Vote.Eligible != 3 {
		t.Fatalf("tie should leave everyone in play and allow a new vote: %+v", v.Vote)
	}

	post("房主", "start-vote", 2, 200, "")
	post("甲", "vote", 3, 200, "乙")
	post("乙", "leave", 3, 200, "")
	if v := getView("房主"); v.Vote.Open || v.Vote.Result.Kind != "cancelled" || !v.Vote.CanStart {
		t.Fatalf("departure should cancel the ballot and let the host restart: %+v", v.Vote)
	}
	host.request("POST", "reset", map[string]any{"code": code, "round": 1}, 200)
	if v := getView("房主"); v.Vote.Number != 0 || v.Vote.Result != nil || v.Players[0].Role != "" {
		t.Fatalf("reset did not clear vote data: %+v", v)
	}
}
