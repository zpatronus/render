package secrethitler

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func members(n int) []member {
	out := make([]member, n)
	for i := range out {
		out[i] = member{ID: fmt.Sprint("p", i), Name: fmt.Sprint("玩家", i+1), Avatar: i + 1}
	}
	return out
}
func fixture(t *testing.T, n int) *game {
	t.Helper()
	g, err := newGame(members(n))
	if err != nil {
		t.Fatal(err)
	}
	g.President = 0
	for i := range g.Players {
		g.Players[i].Role = liberal
	}
	g.Players[1].Role = fascist
	g.Players[n-1].Role = hitler
	return g
}
func perform(t *testing.T, g *game, actor int, action string, target int, card int, yes bool) {
	t.Helper()
	id := ""
	if target >= 0 {
		id = g.Players[target].ID
	}
	if err := g.act(g.Players[actor].ID, action, id, card, yes); err != nil {
		t.Fatalf("%s in %s: %v", action, g.Phase, err)
	}
}
func elect(t *testing.T, g *game, president, chancellor int, yes bool) {
	t.Helper()
	if g.President != president {
		t.Fatalf("wrong president: got %d want %d", g.President, president)
	}
	perform(t, g, president, "nominate", chancellor, 0, false)
	for i, p := range g.Players {
		if p.Alive {
			perform(t, g, i, "vote", -1, 0, yes)
		}
	}
}
func TestOfficialSetupAndInitialInformation(t *testing.T) {
	for n := 5; n <= 10; n++ {
		for run := 0; run < 20; run++ {
			g, err := newGame(members(n))
			if err != nil {
				t.Fatal(err)
			}
			counts := map[role]int{}
			for _, p := range g.Players {
				counts[p.Role]++
			}
			wantF := (n-5)/2 + 1
			if counts[hitler] != 1 || counts[fascist] != wantF || counts[liberal] != n-wantF-1 {
				t.Fatal(n, counts)
			}
			cards := map[policy]int{}
			for _, p := range g.Deck {
				cards[p]++
			}
			if cards[liberalPolicy] != 6 || cards[fascistPolicy] != 11 {
				t.Fatal(cards)
			}
			for _, p := range g.Players {
				v := g.privateView(p.ID)
				if v.Role != p.Role || v.ID != p.ID || v.Party != party(p.Role) {
					t.Fatal("private identity mismatch")
				}
				if p.Role == liberal || (p.Role == hitler && n >= 7) {
					if len(v.Known) != 0 {
						t.Fatal("leaked other identities", n, p.Role)
					}
				} else if len(v.Known) != wantF {
					t.Fatal("wrong fascist knowledge", n, v)
				}
			}
		}
	}
}
func TestTermLimitsUseLastElectedGovernmentAndLivingCount(t *testing.T) {
	g := fixture(t, 7)
	g.LastPresident = 1
	g.LastChancellor = 2
	if g.eligibleChancellor(1) || g.eligibleChancellor(2) || g.eligibleChancellor(0) {
		t.Fatal("term limit or self nomination failure")
	}
	elect(t, g, 0, 3, false)
	if g.LastPresident != 1 || g.LastChancellor != 2 {
		t.Fatal("failed election changed term limits")
	}
	g.Players[5].Alive = false
	g.Players[6].Alive = false
	g.President = 0
	if !g.eligibleChancellor(1) || g.eligibleChancellor(2) {
		t.Fatal("five living players only restrict previous chancellor")
	}
	g.Players[4].Alive = false
	if !g.eligibleChancellor(1) {
		t.Fatal("smaller surviving tables must keep the five-player exception")
	}
}
func TestSimultaneousVotesAndTieFails(t *testing.T) {
	g := fixture(t, 6)
	perform(t, g, 0, "nominate", 2, 0, false)
	perform(t, g, 0, "vote", -1, 0, true)
	if len(g.publicView("p1").LastVotes) != 0 {
		t.Fatal("partial ballots exposed")
	}
	if err := g.act("p0", "vote", "", 0, false); err == nil {
		t.Fatal("vote changed after submission")
	}
	for i := 1; i < 6; i++ {
		perform(t, g, i, "vote", -1, 0, i < 3)
	}
	if g.Phase != nomination || g.Tracker != 1 || g.President != 1 || g.LastVotePassed {
		t.Fatal("3–3 tie must fail")
	}
	if len(g.LastVotes) != 6 {
		t.Fatal("final ballots missing")
	}
}
func TestChaosClearsLimitsSkipsPowerAndStillWins(t *testing.T) {
	for _, winning := range []bool{false, true} {
		g := fixture(t, 9)
		g.Tracker = 2
		g.LastPresident = 3
		g.LastChancellor = 4
		g.FascistPolicies = 0
		if winning {
			g.LiberalPolicies = 4
			g.Deck = []policy{liberalPolicy, fascistPolicy, fascistPolicy, liberalPolicy}
		} else {
			g.Deck = []policy{fascistPolicy, liberalPolicy, fascistPolicy, fascistPolicy}
		}
		elect(t, g, 0, 2, false)
		if g.Tracker != 0 || g.LastPresident != -1 || g.LastChancellor != -1 {
			t.Fatal("chaos did not clear term limits")
		}
		if winning {
			if g.Winner != liberal || g.Phase != finished {
				t.Fatal("chaos must check policy victory")
			}
		} else if g.Phase != nomination || g.Power != "" || g.President != 1 {
			t.Fatal("chaos incorrectly granted an investigation")
		}
	}
}
func TestLegislativePrivacyAndPolicyConservation(t *testing.T) {
	g := fixture(t, 7)
	elect(t, g, 0, 2, true)
	if len(g.privateView("p0").Hand) != 3 || len(g.privateView("p2").Hand) != 0 || len(g.privateView("p1").Hand) != 0 {
		t.Fatal("president hand leaked")
	}
	perform(t, g, 0, "discard", -1, 1, false)
	if len(g.privateView("p0").Hand) != 0 || len(g.privateView("p2").Hand) != 2 {
		t.Fatal("chancellor hand leaked")
	}
	perform(t, g, 2, "enact", -1, 0, false)
	total := len(g.Deck) + len(g.Discard) + len(g.Hand) + g.LiberalPolicies + g.FascistPolicies
	if total != 17 {
		t.Fatal("lost policy cards", total)
	}
	for _, p := range g.Players {
		data, _ := json.Marshal(g.publicView(p.ID))
		for _, key := range []string{`"role"`, `"party"`, `"hand"`, `"deck"`, `"investigations"`} {
			if strings.Contains(string(data), key) {
				t.Fatal("private data in public state", string(data))
			}
		}
	}
}
func TestHitlerElectionWinsBeforeDrawing(t *testing.T) {
	for _, policies := range []int{2, 3, 5} {
		g := fixture(t, 7)
		g.FascistPolicies = policies
		before := len(g.Deck)
		elect(t, g, 0, 6, true)
		if policies >= 3 {
			if g.Winner != fascist || g.Phase != finished || len(g.Deck) != before {
				t.Fatal("Hitler election must end before drawing")
			}
		} else if g.Phase != presidentDiscard {
			t.Fatal("early Hitler election should continue")
		}
	}
}
func TestOfficialExecutiveTracks(t *testing.T) {
	expected := map[int][]power{5: {"", "", peek, execute, execute, ""}, 6: {"", "", peek, execute, execute, ""}, 7: {"", investigate, specialElection, execute, execute, ""}, 8: {"", investigate, specialElection, execute, execute, ""}, 9: {investigate, investigate, specialElection, execute, execute, ""}, 10: {investigate, investigate, specialElection, execute, execute, ""}}
	for n, track := range expected {
		for i, p := range track {
			if got := powerAt(n, i+1); got != p {
				t.Fatal(n, i, got, p)
			}
		}
	}
	g := fixture(t, 9)
	g.Players[7].Alive = false
	g.Players[8].Alive = false
	g.enact(fascistPolicy, false)
	if g.Power != investigate {
		t.Fatal("track must use starting count, not living count")
	}
}
func TestInvestigationRevealsPartyNotHitlerAndCannotRepeat(t *testing.T) {
	g := fixture(t, 9)
	g.Phase = executive
	g.Power = investigate
	perform(t, g, 0, "investigate", 8, 0, false)
	notes := g.privateView("p0").Investigations
	if len(notes) != 1 || notes[0].Party != fascist {
		t.Fatal("Hitler must investigate as fascist")
	}
	if len(g.privateView("p1").Investigations) != 0 {
		t.Fatal("investigation leaked")
	}
	if g.Phase != powerResult {
		t.Fatal("must acknowledge private result")
	}
	perform(t, g, 0, "acknowledge", -1, 0, false)
	g.President = 1
	g.Phase = executive
	g.Power = investigate
	if err := g.act("p1", "investigate", "p8", 0, false); err == nil {
		t.Fatal("repeated investigation allowed")
	}
	if err := g.act("p1", "investigate", "p1", 0, false); err == nil {
		t.Fatal("self investigation allowed")
	}
}
func TestSpecialElectionReturnsToOriginalRotation(t *testing.T) {
	for _, target := range []int{1, 4} {
		g := fixture(t, 7)
		g.Phase = executive
		g.Power = specialElection
		g.LastPresident = 0
		g.LastChancellor = 2
		perform(t, g, 0, "special-election", target, 0, false)
		if g.President != target || g.LastPresident != 0 || g.LastChancellor != 2 {
			t.Fatal("special election reset limits")
		}
		nominee := 3
		if target == 3 {
			nominee = 4
		}
		elect(t, g, target, nominee, false)
		if g.President != 1 {
			t.Fatal("special election skipped normal rotation", target, g.President)
		}
	}
}
func TestPeekKeepsOrderAndIsPrivate(t *testing.T) {
	g := fixture(t, 5)
	g.Phase = executive
	g.Power = peek
	g.Deck = []policy{fascistPolicy, liberalPolicy, fascistPolicy, liberalPolicy}
	before := append([]policy(nil), g.Deck...)
	perform(t, g, 0, "peek", -1, 0, false)
	for i, p := range before {
		if g.Deck[i] != p {
			t.Fatal("peek changed order")
		}
	}
	if len(g.privateView("p0").Peek) != 3 || len(g.privateView("p1").Peek) != 0 {
		t.Fatal("peek privacy failure")
	}
}
func TestExecutionAndFinalRolesAlwaysMatchPrivateRoles(t *testing.T) {
	for _, target := range []int{1, 6} {
		g := fixture(t, 7)
		g.Phase = executive
		g.Power = execute
		original := map[string]role{}
		for _, p := range g.Players {
			original[p.ID] = g.privateView(p.ID).Role
		}
		perform(t, g, 0, "execute", target, 0, false)
		if g.Players[target].Alive {
			t.Fatal("execution did not remove player")
		}
		if target == 1 {
			if g.Winner != "" || len(g.publicView("p0").Results) != 0 {
				t.Fatal("non-Hitler execution revealed roles or ended game")
			}
			if err := g.act("p1", "nominate", "p2", 0, false); err == nil {
				t.Fatal("dead player acted")
			}
			g.end(fascist, "测试结算")
		} else if g.Winner != liberal {
			t.Fatal("executing Hitler must win")
		}
		for _, viewer := range g.Players {
			v := g.publicView(viewer.ID)
			for _, result := range v.Results {
				if result.Role != original[result.ID] || result.Party != party(original[result.ID]) {
					t.Fatal("result role differs from originally assigned role", result)
				}
			}
		}
	}
}
func TestVetoConsentDenialTrackerAndTermLimits(t *testing.T) {
	g := fixture(t, 7)
	g.FascistPolicies = 5
	g.Tracker = 1
	elect(t, g, 0, 2, true)
	perform(t, g, 0, "discard", -1, 0, false)
	perform(t, g, 2, "request-veto", -1, 0, false)
	perform(t, g, 0, "veto", -1, 0, false)
	if g.Phase != chancellorEnact || !g.VetoDenied || g.Tracker != 1 {
		t.Fatal("veto refusal changed tracker or skipped enactment")
	}
	if err := g.act("p2", "request-veto", "", 0, false); err == nil {
		t.Fatal("repeat veto allowed")
	}
	g.Hand = []policy{liberalPolicy, liberalPolicy}
	perform(t, g, 2, "enact", -1, 0, false)
	if g.Tracker != 0 {
		t.Fatal("policy must reset tracker")
	}
	g = fixture(t, 7)
	g.FascistPolicies = 5
	g.Tracker = 1
	elect(t, g, 0, 2, true)
	perform(t, g, 0, "discard", -1, 0, false)
	perform(t, g, 2, "request-veto", -1, 0, false)
	perform(t, g, 0, "veto", -1, 0, true)
	if g.Tracker != 2 || g.LastPresident != 0 || g.LastChancellor != 2 || g.President != 1 {
		t.Fatal("accepted veto lost government term limits or tracker")
	}
	g = fixture(t, 7)
	g.FascistPolicies = 5
	g.Tracker = 2
	elect(t, g, 0, 2, true)
	perform(t, g, 0, "discard", -1, 0, false)
	g.Deck = []policy{fascistPolicy, liberalPolicy, liberalPolicy}
	perform(t, g, 2, "request-veto", -1, 0, false)
	perform(t, g, 0, "veto", -1, 0, true)
	if g.Winner != fascist || g.Tracker != 0 || g.LastPresident != -1 {
		t.Fatal("accepted third veto must trigger chaos and victory")
	}
}
func TestRecycleMixesRemainingDeckAndDiscard(t *testing.T) {
	g := fixture(t, 5)
	g.Deck = []policy{liberalPolicy, fascistPolicy}
	g.Discard = []policy{fascistPolicy, liberalPolicy, fascistPolicy}
	g.recycle()
	if len(g.Deck) != 5 || len(g.Discard) != 0 {
		t.Fatal("reshuffle lost cards")
	}
	n := 0
	for _, p := range g.Deck {
		if p == liberalPolicy {
			n++
		}
	}
	if n != 2 {
		t.Fatal("reshuffle changed policy types")
	}
}
