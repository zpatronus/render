package secrethitler

import "testing"

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
