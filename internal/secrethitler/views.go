// Secret Hitler Chinese adaptation. Licensed under CC BY-NC-SA 4.0; see LICENSE.
package secrethitler

type memberView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar int    `json:"avatar"`
	Bot    bool   `json:"bot"`
	Alive  bool   `json:"alive"`
	Voted  bool   `json:"voted"`
}
type resultView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar int    `json:"avatar"`
	Role   role   `json:"role"`
	Party  role   `json:"party"`
	Alive  bool   `json:"alive"`
}
type gameView struct {
	ID                 string       `json:"id"`
	Step               int          `json:"step"`
	Phase              phase        `json:"phase"`
	Election           int          `json:"election"`
	President          string       `json:"president"`
	Nominee            string       `json:"nominee"`
	LastPresident      string       `json:"lastPresident"`
	LastChancellor     string       `json:"lastChancellor"`
	LiberalPolicies    int          `json:"liberalPolicies"`
	FascistPolicies    int          `json:"fascistPolicies"`
	Tracker            int          `json:"tracker"`
	DeckCount          int          `json:"deckCount"`
	DiscardCount       int          `json:"discardCount"`
	Power              power        `json:"power"`
	Track              []power      `json:"track"`
	VoteCount          int          `json:"voteCount"`
	LastVotes          []ballot     `json:"lastVotes,omitempty"`
	LastVotePresident  string       `json:"lastVotePresident,omitempty"`
	LastVoteChancellor string       `json:"lastVoteChancellor,omitempty"`
	LastVotePassed     bool         `json:"lastVotePassed"`
	Actions            []string     `json:"actions"`
	Targets            []string     `json:"targets"`
	Winner             role         `json:"winner,omitempty"`
	Reason             string       `json:"reason,omitempty"`
	Results            []resultView `json:"results,omitempty"`
	Events             []event      `json:"events"`
}
type roomView struct {
	Code    string       `json:"code"`
	Host    string       `json:"host"`
	You     string       `json:"you"`
	Bots    int          `json:"bots"`
	Members []memberView `json:"members"`
	Game    *gameView    `json:"game,omitempty"`
}

func (r *room) view(id string) roomView {
	v := roomView{Code: r.Code, Host: r.Host, You: id, Bots: r.botCount()}
	if r.Game == nil {
		for _, m := range r.Members {
			v.Members = append(v.Members, memberView{ID: m.ID, Name: m.Name, Avatar: m.Avatar, Bot: m.Bot, Alive: true})
		}
		return v
	}
	g := r.Game
	for _, p := range g.Players {
		_, voted := g.Votes[p.ID]
		v.Members = append(v.Members, memberView{ID: p.ID, Name: p.Name, Avatar: p.Avatar, Bot: p.Bot, Alive: p.Alive, Voted: voted})
	}
	v.Game = g.publicView(id)
	return v
}
func (g *game) publicView(id string) *gameView {
	playerID := func(index int) string {
		if index < 0 {
			return ""
		}
		return g.Players[index].ID
	}
	v := &gameView{ID: g.ID, Step: g.Step, Phase: g.Phase, Election: g.Election, President: playerID(g.President), Nominee: playerID(g.Nominee), LastPresident: playerID(g.LastPresident), LastChancellor: playerID(g.LastChancellor), LiberalPolicies: g.LiberalPolicies, FascistPolicies: g.FascistPolicies, Tracker: g.Tracker, DeckCount: len(g.Deck), DiscardCount: len(g.Discard), Power: g.Power, VoteCount: len(g.Votes), LastVotes: append([]ballot(nil), g.LastVotes...), LastVotePresident: g.LastVotePresident, LastVoteChancellor: g.LastVoteChancellor, LastVotePassed: g.LastVotePassed, Winner: g.Winner, Reason: g.Reason, Events: append([]event(nil), g.Events...), Actions: []string{}, Targets: []string{}}
	for i := 1; i <= 6; i++ {
		v.Track = append(v.Track, powerAt(len(g.Players), i))
	}
	if g.Phase == finished {
		for _, p := range g.Players {
			v.Results = append(v.Results, resultView{ID: p.ID, Name: p.Name, Avatar: p.Avatar, Role: p.Role, Party: party(p.Role), Alive: p.Alive})
		}
		return v
	}
	actor := g.index(id)
	if actor < 0 || !g.Players[actor].Alive {
		return v
	}
	switch g.Phase {
	case nomination:
		if actor == g.President {
			v.Actions = append(v.Actions, "nominate")
			for i, p := range g.Players {
				if g.eligibleChancellor(i) {
					v.Targets = append(v.Targets, p.ID)
				}
			}
		}
	case voting:
		if _, voted := g.Votes[id]; !voted {
			v.Actions = append(v.Actions, "vote")
		}
	case presidentDiscard:
		if actor == g.President {
			v.Actions = append(v.Actions, "discard")
		}
	case chancellorEnact:
		if actor == g.Nominee {
			v.Actions = append(v.Actions, "enact")
			if g.FascistPolicies >= 5 && !g.VetoDenied {
				v.Actions = append(v.Actions, "request-veto")
			}
		}
	case vetoDecision:
		if actor == g.President {
			v.Actions = append(v.Actions, "veto")
		}
	case executive:
		if actor == g.President {
			v.Actions = append(v.Actions, string(g.Power))
			if g.Power != peek {
				for i, p := range g.Players {
					if i != actor && p.Alive && (g.Power != investigate || !g.Investigated[p.ID]) {
						v.Targets = append(v.Targets, p.ID)
					}
				}
			}
		}
	case powerResult:
		if actor == g.President {
			v.Actions = append(v.Actions, "acknowledge")
		}
	}
	return v
}

type knownPlayer struct {
	ID   string `json:"id"`
	Role role   `json:"role"`
}
type privateView struct {
	Game           string          `json:"game"`
	Step           int             `json:"step"`
	ID             string          `json:"id"`
	Role           role            `json:"role"`
	Party          role            `json:"party"`
	Known          []knownPlayer   `json:"known"`
	Hand           []policy        `json:"hand"`
	Investigations []investigation `json:"investigations"`
	Peek           []policy        `json:"peek"`
	PeekElection   int             `json:"peekElection"`
}

func (g *game) privateView(id string) privateView {
	i := g.index(id)
	if i < 0 {
		return privateView{}
	}
	self := g.Players[i]
	v := privateView{Game: g.ID, Step: g.Step, ID: self.ID, Role: self.Role, Party: party(self.Role), Known: []knownPlayer{}, Hand: []policy{}, Investigations: []investigation{}, Peek: []policy{}}
	// Only ordinary Fascists, and Hitler in 5–6 player games, know the other Fascists.
	if self.Role == fascist || (self.Role == hitler && len(g.Players) <= 6) {
		for _, p := range g.Players {
			if p.ID != id && party(p.Role) == fascist {
				v.Known = append(v.Known, knownPlayer{ID: p.ID, Role: p.Role})
			}
		}
	}
	if self.Alive && ((g.Phase == presidentDiscard && i == g.President) || (g.Phase == chancellorEnact && i == g.Nominee)) {
		v.Hand = append(v.Hand, g.Hand...)
	}
	if notes := g.Notes[id]; notes != nil {
		v.Investigations = append(v.Investigations, notes.Investigations...)
		v.Peek = append(v.Peek, notes.Peek...)
		v.PeekElection = notes.PeekElection
	}
	return v
}
