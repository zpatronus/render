package secrethitler

import (
	"testing"
	"time"
)

func TestBotsCanPlayWholeOfficialGamesWithoutHumans(t *testing.T) {
	for n := 5; n <= 10; n++ {
		for run := 0; run < 12; run++ {
			people := members(n)
			for i := range people {
				people[i].Bot = true
			}
			g, err := newGame(people)
			if err != nil {
				t.Fatal(err)
			}
			for turn := 0; turn < 20 && g.Phase != finished; turn++ {
				advanceBots(g)
			}
			if g.Phase != finished || g.Winner == "" {
				t.Fatal("bots stalled", n, g.Phase)
			}
			total := len(g.Deck) + len(g.Discard) + len(g.Hand) + g.LiberalPolicies + g.FascistPolicies
			if total != 17 {
				t.Fatal("bot game lost cards", n, total)
			}
			for _, p := range g.Players {
				private := g.privateView(p.ID)
				for _, result := range g.publicView(p.ID).Results {
					if result.ID == p.ID && private.Role != result.Role {
						t.Fatal("bot role differs at game end")
					}
				}
			}
		}
	}
}
func TestBotsUseOnlyTheirLegitimatePrivateInformation(t *testing.T) {
	public := &gameView{Actions: []string{"nominate"}, Targets: []string{"unknown", "hitler"}, FascistPolicies: 3}
	private := privateView{ID: "self", Role: fascist, Known: []knownPlayer{{ID: "hitler", Role: hitler}}}
	move, ok := botDecision(public, private)
	if !ok || move.Target != "hitler" {
		t.Fatal("known fascist should nominate Hitler after third policy", move)
	}
	public.Actions = []string{"execute"}
	move, ok = botDecision(public, private)
	if !ok || move.Target != "unknown" {
		t.Fatal("known Hitler should be protected", move)
	}
	public.Targets = []string{"hitler"}
	if _, ok = botDecision(public, private); !ok {
		t.Fatal("mandatory power must not stall with only a known ally eligible")
	}
	public.Actions = []string{"discard"}
	private.Hand = []policy{liberalPolicy, fascistPolicy, fascistPolicy}
	move, _ = botDecision(public, private)
	if move.Card != 0 {
		t.Fatal("fascist did not deliberately discard liberal policy")
	}
	public.Actions = []string{"enact"}
	private.Role = liberal
	private.Hand = []policy{fascistPolicy, liberalPolicy}
	move, _ = botDecision(public, private)
	if move.Card != 1 {
		t.Fatal("liberal did not deliberately select liberal policy")
	}
}

func TestRoomBotsWaitAndPublishOnlyOneMoveAtATime(t *testing.T) {
	g := fixture(t, 5)
	for i := range g.Players {
		g.Players[i].Bot = true
	}
	r := &room{Code: "ABC123", Game: g}
	s := &server{rooms: map[string]*room{r.Code: r}, now: time.Now}
	defer func() { s.mu.Lock(); delete(s.rooms, r.Code); s.mu.Unlock() }()
	before := g.Step
	s.mu.Lock()
	s.advanceRoomBots(r)
	if g.Step != before {
		t.Fatal("scheduling executed a synchronous bot move")
	}
	s.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	s.mu.Lock()
	early := g.Step
	s.mu.Unlock()
	if early != before {
		t.Fatal("bot skipped thinking interval")
	}
	deadline := time.Now().Add(botActionDelay + time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		step, phase, votes := g.Step, g.Phase, len(g.Votes)
		s.mu.Unlock()
		if step != before {
			if phase != voting || votes != 0 {
				t.Fatal("bot ran several operations at once", phase, votes)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("delayed bot never acted")
}
