package rules

import "poolarena/game-server/internal/physics"

type Group string

const (
	Open    Group = "open"
	Reds    Group = "reds"
	Yellows Group = "yellows"
)

type State struct {
	Groups [2]Group
	Break  bool
}
type ShotCall struct {
	CalledBall   int
	CalledPocket int
	Safety       bool
}
type BreakChoice struct {
	Required bool
	Actor    int
	Kind     string
	Options  []string
}
type Outcome struct {
	Foul               bool        `json:"foul"`
	FoulCode           string      `json:"foulCode,omitempty"`
	Continue           bool        `json:"continue"`
	NextPlayer         int         `json:"nextPlayer"`
	BallInHand         bool        `json:"ballInHand"`
	BallInHandHeadOnly bool        `json:"ballInHandHeadOnly"`
	Winner             int         `json:"winner"`
	Loser              int         `json:"loser"`
	Groups             [2]Group    `json:"groups"`
	BreakChoice        BreakChoice `json:"breakChoice"`
}

func Evaluate(st State, shooter int, call ShotCall, before, after []physics.Ball, rep physics.ShotReport) Outcome {
	other := 1 - shooter
	out := Outcome{NextPlayer: other, Winner: -1, Loser: -1, Groups: st.Groups}
	if st.Break {
		return evaluateBreak(st, shooter, before, after, rep)
	}

	eightPocketed := contains(rep.Pocketed, 8)
	eightOff := contains(rep.BallsOffTable, 8)
	foul, code := standardFoul(st, shooter, before, rep)
	out.Foul = foul
	out.FoulCode = code

	// Eight-ball loss/win conditions override ordinary turn continuation.
	if eightPocketed || eightOff {
		if eightOff || foul || !groupClearedBefore(st.Groups[shooter], before) || call.Safety {
			out.Winner = other
			out.Loser = shooter
			out.Continue = false
			return out
		}
		out.Winner = shooter
		out.Loser = other
		out.Continue = false
		return out
	}

	if foul {
		out.BallInHand = true
		out.Continue = false
		return out
	}

	pocketedGroup := firstPocketedGroup(rep.Pocketed)
	if st.Groups[shooter] == Open && pocketedGroup != Open {
		g := pocketedGroup
		if g != Open {
			out.Groups[shooter] = g
			out.Groups[other] = opposite(g)
		}
	}
	if call.Safety {
		out.Continue = false
		return out
	}
	if g := out.Groups[shooter]; g != Open && pocketedFromGroup(rep.Pocketed, g) {
		out.Continue = true
		out.NextPlayer = shooter
		return out
	}
	return out
}

func evaluateBreak(st State, shooter int, before, after []physics.Ball, rep physics.ShotReport) Outcome {
	other := 1 - shooter
	out := Outcome{NextPlayer: other, Winner: -1, Loser: -1, Groups: st.Groups}
	eight := contains(rep.Pocketed, 8)
	objectPocketed := false
	for _, id := range rep.Pocketed {
		if id != 0 {
			objectPocketed = true
			break
		}
	}
	foul := rep.CueScratch || rep.FirstObjectBall < 0 || hasObjectOff(rep.BallsOffTable)
	if eight {
		if foul {
			out.Foul = true
			out.FoulCode = breakFoulCode(rep)
			out.BreakChoice = BreakChoice{Required: true, Actor: other, Kind: "eight_on_foul", Options: []string{"spot_8_bih_head", "rerack_opponent_break"}}
			return out
		}
		out.BreakChoice = BreakChoice{Required: true, Actor: shooter, Kind: "eight_legal", Options: []string{"spot_8_continue", "rerack_self_break"}}
		out.NextPlayer = shooter
		return out
	}
	if foul {
		out.Foul = true
		out.FoulCode = breakFoulCode(rep)
		out.BreakChoice = BreakChoice{Required: true, Actor: other, Kind: "break_foul", Options: []string{"accept_table", "ball_in_hand_head"}}
		return out
	}
	if !objectPocketed {
		return out
	}
	out.Continue = true
	out.NextPlayer = shooter
	return out
}

func standardFoul(st State, shooter int, before []physics.Ball, rep physics.ShotReport) (bool, string) {
	if rep.CueScratch {
		return true, "scratch"
	}
	if contains(rep.BallsOffTable, 0) {
		return true, "cue_ball_off_table"
	}
	if hasObjectOff(rep.BallsOffTable) {
		return true, "object_ball_off_table"
	}
	if rep.FirstObjectBall < 0 {
		return true, "no_object_contact"
	}
	g := st.Groups[shooter]
	if g == Open {
		if rep.FirstObjectBall == 8 {
			return true, "wrong_ball_first"
		}
	} else if !groupClearedBefore(g, before) && !ballBelongs(rep.FirstObjectBall, g) {
		return true, "wrong_ball_first"
	} else if groupClearedBefore(g, before) && rep.FirstObjectBall != 8 {
		return true, "wrong_ball_first"
	}
	if len(rep.Pocketed) == 0 && !rep.AnyRailAfterFirst {
		return true, "no_rail_after_contact"
	}
	return false, ""
}

func groupClearedBefore(g Group, balls []physics.Ball) bool {
	if g == Open {
		return false
	}
	return allGroupGone(g, balls)
}
func allGroupGone(g Group, balls []physics.Ball) bool {
	for _, b := range balls {
		if b.State == physics.BallOnTable && ballBelongs(b.ID, g) {
			return false
		}
	}
	return true
}
func groupForBall(id int) Group {
	if id >= 1 && id <= 7 {
		return Reds
	}
	if id >= 9 && id <= 15 {
		return Yellows
	}
	return Open
}
func opposite(g Group) Group {
	if g == Reds {
		return Yellows
	}
	if g == Yellows {
		return Reds
	}
	return Open
}
func ballBelongs(id int, g Group) bool {
	return (g == Reds && id >= 1 && id <= 7) || (g == Yellows && id >= 9 && id <= 15)
}
func firstPocketedGroup(ids []int) Group {
	for _, id := range ids {
		if g := groupForBall(id); g != Open {
			return g
		}
	}
	return Open
}
func pocketedFromGroup(ids []int, g Group) bool {
	for _, id := range ids {
		if ballBelongs(id, g) {
			return true
		}
	}
	return false
}
func contains(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func hasObjectOff(a []int) bool {
	for _, x := range a {
		if x != 0 {
			return true
		}
	}
	return false
}
func breakFoulCode(rep physics.ShotReport) string {
	if rep.CueScratch {
		return "break_scratch"
	}
	if rep.FirstObjectBall < 0 {
		return "break_no_contact"
	}
	if hasObjectOff(rep.BallsOffTable) {
		return "break_ball_off_table"
	}
	return "break_foul"
}
