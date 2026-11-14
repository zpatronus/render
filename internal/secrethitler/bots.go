// Secret Hitler Chinese adaptation. Licensed under CC BY-NC-SA 4.0; see LICENSE.
package secrethitler

import "fmt"

func (r *room) botCount() int {
	minimum := max(0, 5-len(r.Members))
	maximum := max(0, 10-len(r.Members))
	if r.Bots < 0 {
		return minimum
	}
	return min(max(r.Bots, minimum), maximum)
}
func (r *room) gameMembers() []member {
	players := append([]member(nil), r.Members...)
	for i := 0; i < r.botCount(); i++ {
		name := fmt.Sprintf("机器人 %d", i+1)
		for {
			found := false
			for _, p := range players {
				if p.Name == name {
					found = true
					break
				}
			}
			if !found {
				break
			}
			name += "号"
		}
		players = append(players, member{ID: "bot-" + randomID(8), Name: name, Avatar: (len(players) % 20) + 1, Bot: true})
	}
	return players
}

type botMove struct {
	Action, Target string
	Card           int
	Yes            bool
}

// Decisions receive only the same public/private views available to a player.
// No access to the hidden deck, other hands, or unearned role information.
func botDecision(public *gameView, private privateView) (botMove, bool) {
	if len(public.Actions) == 0 {
		return botMove{}, false
	}
	action := public.Actions[0]
	move := botMove{Action: action}
	known := map[string]role{private.ID: private.Role}
	for _, p := range private.Known {
		known[p.ID] = p.Role
	}
	for _, p := range private.Investigations {
		if _, exists := known[p.ID]; !exists {
			known[p.ID] = p.Party
		}
	}
	myParty := party(private.Role)
	switch action {
	case "vote":
		if known[public.Nominee] == hitler && public.FascistPolicies >= 3 {
			move.Yes = myParty == fascist
			return move, true
		}
		if nominee, ok := known[public.Nominee]; ok {
			move.Yes = party(nominee) == myParty
			return move, true
		}
		// Unknown candidates: allow most governments, occasionally vote Nein.
		move.Yes = randomIndex(5) != 0
	case "discard", "enact":
		if len(private.Hand) == 0 {
			return botMove{}, false
		}
		wanted := liberalPolicy
		if myParty == fascist && (private.Role != hitler || public.FascistPolicies >= 3) {
			wanted = fascistPolicy
		}
		for i, p := range private.Hand {
			if (action == "enact" && p == wanted) || (action == "discard" && p != wanted) {
				move.Card = i
				return move, true
			}
		}
		// Deliberately choose the first card if all options are equivalent.
		move.Card = 0
	case "veto":
		move.Yes = myParty == liberal
	case "peek", "acknowledge":
	case "nominate", "special-election", "investigate", "execute":
		if len(public.Targets) == 0 {
			return botMove{}, false
		}
		best := -10000
		var choices []string
		for _, id := range public.Targets {
			score := 0
			r, knownPlayer := known[id]
			if action == "investigate" {
				if !knownPlayer {
					score += 3
				}
			} else if action == "execute" {
				if knownPlayer {
					if party(r) == myParty {
						score -= 10
					} else {
						score += 10
					}
				}
				if myParty == fascist && r == hitler {
					score -= 100
				}
			} else {
				if knownPlayer {
					if party(r) == myParty {
						score += 5
					} else {
						score -= 5
					}
				}
				if action == "nominate" && myParty == fascist && r == hitler && public.FascistPolicies >= 3 {
					score += 100
				}
			}
			if score > best {
				best = score
				choices = []string{id}
			} else if score == best {
				choices = append(choices, id)
			}
		}
		move.Target = choices[randomIndex(len(choices))]
	default:
		return botMove{}, false
	}
	return move, true
}
func advanceBots(g *game) {
	// Bounded work per request. Further polling continues an all-bot endgame.
	for actions := 0; actions < 160 && g.Phase != finished; actions++ {
		moved := false
		for _, p := range g.Players {
			if !p.Bot || !p.Alive {
				continue
			}
			move, ok := botDecision(g.publicView(p.ID), g.privateView(p.ID))
			if !ok {
				continue
			}
			if err := g.act(p.ID, move.Action, move.Target, move.Card, move.Yes); err != nil {
				continue
			}
			moved = true
			break
		}
		if !moved {
			return
		}
	}
}
