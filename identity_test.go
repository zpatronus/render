package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPINRestoresSeatsAcrossBrowsers(t *testing.T) {
	h := handler()
	call := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		q := httptest.NewRequest("POST", path, strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, q)
		return w
	}
	for _, game := range []string{"abracada", "undercover", "secret-hitler", "formula-war"} {
		t.Run(game, func(t *testing.T) {
			path := "/" + game + "/api/create"
			body := `{"name":"法师","avatar":1,"pin":"1234"}`
			if game == "formula-war" {
				path = "/formula-war/api/rooms"
				body = `{"name":"PIN test","player":"法师","pin":"1234"}`
			}
			created := call(path, body)
			if created.Code != 201 {
				t.Fatal(created.Code, created.Body.String())
			}
			var v map[string]json.RawMessage
			json.Unmarshal(created.Body.Bytes(), &v)
			var room, player string
			if game == "formula-war" {
				var r struct{ ID string }
				json.Unmarshal(v["room"], &r)
				room = r.ID
				json.Unmarshal(v["player"], &player)
			} else {
				json.Unmarshal(v["code"], &room)
				json.Unmarshal(v["you"], &player)
			}
			path = "/" + game + "/api/join"
			body = fmt.Sprintf(`{"code":%q,"name":"法师","avatar":1,"pin":"1234"}`, room)
			if game == "formula-war" {
				path = "/formula-war/api/rooms/" + room + "/join"
				body = `{"name":"法师","pin":"1234"}`
			}
			if wrong := call(path, strings.ReplaceAll(body, "1234", "9999")); wrong.Code != 403 {
				t.Fatal("wrong PIN accepted", wrong.Body.String())
			}
			joined := call(path, body)
			if joined.Code != 200 {
				t.Fatal(joined.Code, joined.Body.String())
			}
			json.Unmarshal(joined.Body.Bytes(), &v)
			var recovered string
			if game == "formula-war" {
				json.Unmarshal(v["player"], &recovered)
			} else {
				json.Unmarshal(v["you"], &recovered)
			}
			if recovered != player {
				t.Fatal("seat changed")
			}
			changed := call("/"+game+"/api/pin", fmt.Sprintf(`{"code":%q,"username":"法师","pin":"1234","newPin":"5678"}`, room))
			if changed.Code != 200 {
				t.Fatal(changed.Code, changed.Body.String())
			}
			if old := call(path, body); old.Code != 403 {
				t.Fatal("old PIN still accepted")
			}
			if updated := call(path, strings.ReplaceAll(body, "1234", "5678")); updated.Code != 200 {
				t.Fatal("new PIN rejected", updated.Body.String())
			}
		})
	}
}

func TestLastPlayerLeavingKeepsRoomJoinable(t *testing.T) {
	h := handler()
	call := func(path string, data map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(data)
		q := httptest.NewRequest("POST", path, strings.NewReader(string(raw)))
		q.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, q)
		return w
	}
	for _, game := range []string{"abracada", "undercover", "secret-hitler", "formula-war"} {
		t.Run(game, func(t *testing.T) {
			base := "/" + game + "/api/"
			path := base + "create"
			data := map[string]any{"name": "First", "pin": "1234", "avatar": 1}
			if game == "formula-war" {
				path = base + "rooms"
				data["player"] = "First"
				data["name"] = "Empty room test"
			}
			created := call(path, data)
			if created.Code != 201 {
				t.Fatal(created.Code, created.Body.String())
			}
			var v struct {
				Code string
				Room struct{ ID string }
			}
			json.Unmarshal(created.Body.Bytes(), &v)
			code := v.Code
			if game == "formula-war" {
				code = v.Room.ID
			}
			path = base + "leave"
			if game == "formula-war" {
				path = base + "rooms/" + code + "/leave"
			}
			left := call(path, map[string]any{"code": code, "username": "First", "pin": "1234"})
			if left.Code != 200 {
				t.Fatal(left.Code, left.Body.String())
			}
			path = base + "join"
			if game == "formula-war" {
				path = base + "rooms/" + code + "/join"
			}
			joined := call(path, map[string]any{"code": code, "name": "Second", "pin": "5678", "avatar": 1})
			if joined.Code != 200 {
				t.Fatal("empty room cannot be joined", joined.Code, joined.Body.String())
			}
			var recovered struct {
				Host, You, Player string
				Room              struct{ Host string }
			}
			json.Unmarshal(joined.Body.Bytes(), &recovered)
			if game == "formula-war" {
				if recovered.Room.Host != recovered.Player {
					t.Fatal("new player not host")
				}
			} else if recovered.Host != recovered.You {
				t.Fatal("new player not host")
			}
		})
	}
}
