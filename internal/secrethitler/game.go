// Secret Hitler Chinese adaptation. Licensed under CC BY-NC-SA 4.0; see LICENSE.
package secrethitler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
)

type role string

const (
	liberal role = "liberal"
	fascist role = "fascist"
	hitler  role = "hitler"
)

type policy string

const (
	liberalPolicy policy = "liberal"
	fascistPolicy policy = "fascist"
)

type phase string

const (
	nomination       phase = "nomination"
	voting           phase = "voting"
	presidentDiscard phase = "president-discard"
	chancellorEnact  phase = "chancellor-enact"
	vetoDecision     phase = "veto-decision"
	executive        phase = "executive"
	powerResult      phase = "power-result"
	finished         phase = "finished"
)

type power string

const (
	investigate     power = "investigate"
	specialElection power = "special-election"
	peek            power = "peek"
	execute         power = "execute"
)

type member struct {
	ID, Name string
	Avatar   int
	Bot      bool
}
type player struct {
	member
	Role  role
	Alive bool
}
type ballot struct {
	ID  string `json:"id"`
	Yes bool   `json:"yes"`
}
type investigation struct {
	ID    string `json:"id"`
	Party role   `json:"party"`
}
type privateNotes struct {
	Investigations []investigation
	Peek           []policy
	PeekElection   int
}
type event struct {
	ID       int    `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Policy   policy `json:"policy,omitempty"`
	Target   string `json:"target,omitempty"`
	Winner   role   `json:"winner,omitempty"`
	Election int    `json:"election"`
	Text     string `json:"text"`
}
type game struct {
	ID                                        string
	Players                                   []player
	Phase                                     phase
	Step, Election                            int
	President, Nominee                        int
	LastPresident, LastChancellor             int
	SpecialReturn                             int
	LiberalPolicies, FascistPolicies, Tracker int
	Deck, Discard, Hand                       []policy
	Votes                                     map[string]bool
	LastVotes                                 []ballot
	LastVotePresident, LastVoteChancellor     string
	LastVotePassed                            bool
	Investigated                              map[string]bool
	Notes                                     map[string]*privateNotes
	Power                                     power
	VetoDenied                                bool
	Winner                                    role
	Reason                                    string
	Events                                    []event
	EventSequence                             int
}

func randomIndex(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}
func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func shuffle[T any](items []T) {
	for i := len(items) - 1; i > 0; i-- {
		j := randomIndex(i + 1)
		items[i], items[j] = items[j], items[i]
	}
}
func newGame(members []member) (*game, error) {
	if len(members) < 5 || len(members) > 10 {
		return nil, errors.New("需要 5–10 名玩家")
	}
	g := &game{ID: randomID(12), Phase: nomination, Step: 1, Election: 1, Nominee: -1, LastPresident: -1, LastChancellor: -1, SpecialReturn: -1, Investigated: map[string]bool{}, Notes: map[string]*privateNotes{}}
	roles := make([]role, len(members))
	for i := range roles {
		roles[i] = liberal
	}
	roles[0] = hitler
	ordinaryFascists := (len(members)-5)/2 + 1
	for i := 1; i <= ordinaryFascists; i++ {
		roles[i] = fascist
	}
	shuffle(roles)
	for i, m := range members {
		g.Players = append(g.Players, player{member: m, Role: roles[i], Alive: true})
		g.Notes[m.ID] = &privateNotes{}
	}
	for i := 0; i < 17; i++ {
		p := fascistPolicy
		if i < 6 {
			p = liberalPolicy
		}
		g.Deck = append(g.Deck, p)
	}
	shuffle(g.Deck)
	g.President = randomIndex(len(members))
	g.log("游戏开始，首位总统候选人为 " + g.Players[g.President].Name)
	return g, nil
}
func (g *game) log(text string) {
	g.EventSequence++
	g.Events = append(g.Events, event{ID: g.EventSequence, Election: g.Election, Text: text})
	if len(g.Events) > 120 {
		g.Events = append([]event(nil), g.Events[len(g.Events)-120:]...)
	}
}
func (g *game) index(id string) int {
	for i, p := range g.Players {
		if p.ID == id {
			return i
		}
	}
	return -1
}
func (g *game) aliveCount() int {
	n := 0
	for _, p := range g.Players {
		if p.Alive {
			n++
		}
	}
	return n
}
func (g *game) setPhase(p phase) { g.Phase = p; g.Step++ }
func (g *game) nextAlive(after int) int {
	for n := 1; n <= len(g.Players); n++ {
		i := (after + n) % len(g.Players)
		if g.Players[i].Alive {
			return i
		}
	}
	panic("no living players")
}
func (g *game) eligibleChancellor(i int) bool {
	if i < 0 || i >= len(g.Players) || !g.Players[i].Alive || i == g.President || i == g.LastChancellor {
		return false
	}
	return g.aliveCount() <= 5 || i != g.LastPresident
}
func (g *game) nextElection() {
	if g.SpecialReturn >= 0 {
		g.President = g.nextAlive(g.SpecialReturn)
		g.SpecialReturn = -1
	} else {
		g.President = g.nextAlive(g.President)
	}
	g.Nominee = -1
	g.Hand = nil
	g.Power = ""
	g.Votes = nil
	g.VetoDenied = false
	g.Election++
	g.setPhase(nomination)
}
func (g *game) recycle() {
	if len(g.Deck) < 3 {
		g.Deck = append(g.Deck, g.Discard...)
		g.Discard = nil
		shuffle(g.Deck)
	}
}
func (g *game) draw(n int) []policy {
	if len(g.Deck) < n {
		g.recycle()
	}
	if len(g.Deck) < n {
		panic("policy conservation violated")
	}
	cards := append([]policy(nil), g.Deck[:n]...)
	g.Deck = g.Deck[n:]
	return cards
}
func (g *game) end(winner role, reason string) {
	g.Winner = winner
	g.Reason = reason
	g.Hand = nil
	g.Power = ""
	g.setPhase(finished)
	g.log(reason)
	g.Events[len(g.Events)-1].Kind = "victory"
	g.Events[len(g.Events)-1].Winner = winner
}
func powerAt(players, policies int) power {
	if policies == 4 || policies == 5 {
		return execute
	}
	if players <= 6 {
		if policies == 3 {
			return peek
		}
		return ""
	}
	if policies == 3 {
		return specialElection
	}
	if policies == 2 || (players >= 9 && policies == 1) {
		return investigate
	}
	return ""
}
func (g *game) enact(p policy, chaos bool) {
	g.Tracker = 0
	if p == liberalPolicy {
		g.LiberalPolicies++
		g.log("颁布自由派政策")
	} else {
		g.FascistPolicies++
		g.log("颁布法西斯政策")
	}
	g.Events[len(g.Events)-1].Kind = "policy"
	g.Events[len(g.Events)-1].Policy = p
	g.recycle()
	if g.LiberalPolicies == 5 {
		g.end(liberal, "已颁布 5 项自由派政策")
		return
	}
	if g.FascistPolicies == 6 {
		g.end(fascist, "已颁布 6 项法西斯政策")
		return
	}
	if !chaos && p == fascistPolicy {
		g.Power = powerAt(len(g.Players), g.FascistPolicies)
		if g.Power != "" {
			g.setPhase(executive)
			return
		}
	}
	g.nextElection()
}
func (g *game) failedGovernment() {
	g.Tracker++
	if g.Tracker == 3 {
		g.log("选举计数达到 3：自动颁布牌堆顶政策，不执行总统权力")
		g.LastPresident = -1
		g.LastChancellor = -1
		g.enact(g.draw(1)[0], true)
		return
	}
	g.nextElection()
}
func party(r role) role {
	if r == hitler {
		return fascist
	}
	return r
}

// act is the sole authority for state transitions; callers cannot supply roles or policies.
func (g *game) act(actorID, action, targetID string, card int, yes bool) error {
	actor := g.index(actorID)
	if actor < 0 || !g.Players[actor].Alive {
		return errors.New("你已出局，不能操作")
	}
	if g.Phase == finished {
		return errors.New("本局已经结束")
	}
	target := g.index(targetID)
	switch action {
	case "nominate":
		if g.Phase != nomination || actor != g.President {
			return errors.New("只有总统候选人可以提名")
		}
		if !g.eligibleChancellor(target) {
			return errors.New("该玩家不符合总理候选资格")
		}
		g.Nominee = target
		g.Votes = map[string]bool{}
		g.log(g.Players[actor].Name + " 提名 " + g.Players[target].Name + " 为总理候选人")
		g.setPhase(voting)
	case "vote":
		if g.Phase != voting {
			return errors.New("当前不是投票阶段")
		}
		if _, ok := g.Votes[actorID]; ok {
			return errors.New("你已提交选票")
		}
		g.Votes[actorID] = yes
		if len(g.Votes) < g.aliveCount() {
			return nil
		}
		approvals := 0
		g.LastVotes = nil
		for _, p := range g.Players {
			if p.Alive {
				vote := g.Votes[p.ID]
				g.LastVotes = append(g.LastVotes, ballot{ID: p.ID, Yes: vote})
				if vote {
					approvals++
				}
			}
		}
		g.LastVotePresident = g.Players[g.President].ID
		g.LastVoteChancellor = g.Players[g.Nominee].ID
		g.LastVotePassed = approvals > g.aliveCount()/2
		if !g.LastVotePassed {
			g.log("政府选举未通过")
			g.failedGovernment()
			return nil
		}
		g.LastPresident = g.President
		g.LastChancellor = g.Nominee
		g.log("政府选举通过")
		if g.FascistPolicies >= 3 && g.Players[g.Nominee].Role == hitler {
			g.end(fascist, "3 项法西斯政策后，希特勒当选总理")
			return nil
		}
		g.Hand = g.draw(3)
		g.setPhase(presidentDiscard)
	case "discard":
		if g.Phase != presidentDiscard || actor != g.President {
			return errors.New("只有总统可以弃置这张政策")
		}
		if card < 0 || card >= len(g.Hand) {
			return errors.New("请选择一张政策")
		}
		g.Discard = append(g.Discard, g.Hand[card])
		g.Hand = append(g.Hand[:card], g.Hand[card+1:]...)
		g.setPhase(chancellorEnact)
	case "enact":
		if g.Phase != chancellorEnact || actor != g.Nominee {
			return errors.New("只有总理可以颁布政策")
		}
		if card < 0 || card >= len(g.Hand) {
			return errors.New("请选择一张政策")
		}
		chosen := g.Hand[card]
		g.Discard = append(g.Discard, g.Hand[1-card])
		g.Hand = nil
		g.enact(chosen, false)
	case "request-veto":
		if g.Phase != chancellorEnact || actor != g.Nominee || g.FascistPolicies < 5 || g.VetoDenied {
			return errors.New("当前不能提出否决")
		}
		g.log("总理请求否决本次议程")
		g.setPhase(vetoDecision)
	case "veto":
		if g.Phase != vetoDecision || actor != g.President {
			return errors.New("只有总统可以回应否决")
		}
		if !yes {
			g.VetoDenied = true
			g.log("总统拒绝否决，总理必须颁布政策")
			g.setPhase(chancellorEnact)
			return nil
		}
		g.Discard = append(g.Discard, g.Hand...)
		g.Hand = nil
		g.recycle()
		g.log("总统同意否决，选举计数增加 1")
		g.failedGovernment()
	case "investigate", "special-election", "execute", "peek":
		if g.Phase != executive || actor != g.President || power(action) != g.Power {
			return errors.New("当前不能使用该总统权力")
		}
		if action != "peek" && (target < 0 || target == actor || !g.Players[target].Alive) {
			return errors.New("请选择另一名在场玩家")
		}
		switch action {
		case "investigate":
			id := g.Players[target].ID
			if g.Investigated[id] {
				return errors.New("每名玩家整局只能被调查一次")
			}
			g.Investigated[id] = true
			g.Notes[actorID].Investigations = append(g.Notes[actorID].Investigations, investigation{ID: id, Party: party(g.Players[target].Role)})
			g.log(g.Players[actor].Name + " 调查了 " + g.Players[target].Name + " 的党派")
			g.setPhase(powerResult)
		case "peek":
			g.Notes[actorID].Peek = append([]policy(nil), g.Deck[:3]...)
			g.Notes[actorID].PeekElection = g.Election
			g.log(g.Players[actor].Name + " 查看了牌堆顶的三张政策")
			g.setPhase(powerResult)
		case "special-election":
			g.log(g.Players[actor].Name + " 指定 " + g.Players[target].Name + " 主持特别选举")
			g.SpecialReturn = g.President
			g.President = target
			g.Nominee = -1
			g.Power = ""
			g.Election++
			g.setPhase(nomination)
		case "execute":
			g.Players[target].Alive = false
			g.log(g.Players[actor].Name + " 处决了 " + g.Players[target].Name)
			g.Events[len(g.Events)-1].Kind = "execution"
			g.Events[len(g.Events)-1].Target = g.Players[target].ID
			if g.Players[target].Role == hitler {
				g.end(liberal, "希特勒被处决")
			} else {
				g.nextElection()
			}
		}
	case "acknowledge":
		if g.Phase != powerResult || actor != g.President {
			return errors.New("只有总统可以确认私密结果")
		}
		g.nextElection()
	default:
		return errors.New("未知操作")
	}
	return nil
}
