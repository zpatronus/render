package undercover

type votingState struct {
	number  int
	ballots map[string]string // voter ID to target ID, kept private
	result  *voteResult
}

type voteResult struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"`
}

type voteView struct {
	Number   int         `json:"number"`
	Open     bool        `json:"open"`
	CanStart bool        `json:"canStart"`
	Cast     int         `json:"cast"`
	Eligible int         `json:"eligible"`
	Choice   string      `json:"choice,omitempty"`
	Result   *voteResult `json:"result,omitempty"`
}

func (r *room) eligibleVoters() int {
	count := 0
	for _, p := range r.players {
		if !p.eliminated {
			count++
		}
	}
	return count
}

func (r *room) canStartVote() bool {
	return r.dealt && r.voting.ballots == nil && r.eligibleVoters() >= 2
}

func (r *room) voteView(id string) voteView {
	v := voteView{
		Number: r.voting.number, Open: r.voting.ballots != nil,
		CanStart: r.host == id && r.canStartVote(), Cast: len(r.voting.ballots),
		Eligible: r.eligibleVoters(), Result: r.voting.result,
	}
	if target, ok := r.voting.ballots[id]; ok {
		for _, p := range r.players {
			if p.id == target {
				v.Choice = p.name
				break
			}
		}
	}
	return v
}

func (r *room) castVote(index int, targetName string) (int, string) {
	voter := r.players[index]
	if voter.eliminated {
		return 403, "已出局的玩家不能投票"
	}
	if r.voting.ballots == nil {
		return 409, "当前没有进行中的投票"
	}
	if _, voted := r.voting.ballots[voter.id]; voted {
		return 409, "你的投票已锁定"
	}
	for _, target := range r.players {
		if target.name == targetName && target.id != voter.id && !target.eliminated {
			r.voting.ballots[voter.id] = target.id
			if len(r.voting.ballots) == r.eligibleVoters() {
				r.finishVote()
			}
			return 0, ""
		}
	}
	return 400, "请选择另一位仍在局中的玩家"
}

func (r *room) finishVote() {
	totals := make(map[string]int)
	for _, target := range r.voting.ballots {
		totals[target]++
	}
	highest, winner, tied := 0, -1, false
	for i, p := range r.players {
		count := totals[p.id]
		if count > highest {
			highest, winner, tied = count, i, false
		} else if count == highest {
			tied = true
		}
	}
	result := voteResult{Kind: "tie"}
	if winner >= 0 && !tied {
		p := &r.players[winner]
		p.eliminated = true
		result = voteResult{Kind: "eliminated", Name: p.name, Role: p.role}
	}
	r.voting.ballots = nil
	r.voting.result = &result
}
