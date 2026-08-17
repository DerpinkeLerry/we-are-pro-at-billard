package rules_test

import (
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
	"testing"
)

func balls(ids ...int) []physics.Ball {
	out := make([]physics.Ball, 0, len(ids))
	for _, id := range ids {
		out = append(out, physics.Ball{ID: id, State: physics.BallOnTable})
	}
	return out
}

func fullTable() []physics.Ball {
	ids := make([]int, 16)
	for i := range ids {
		ids[i] = i
	}
	return balls(ids...)
}

func TestWrongBallFirstIsFoul(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 9, AnyRailAfterFirst: true, PocketByBall: map[int]int{}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 8, 9), balls(0, 1, 8, 9), rep)
	if !o.Foul || !o.BallInHand {
		t.Fatalf("expected foul %+v", o)
	}
}

func TestAssignsOpenTableToFirstPocketedColour(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: 3, Pocketed: []int{3}, PocketByBall: map[int]int{3: 2}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 3, 8, 10), balls(0, 8, 10), rep)
	if o.Groups[0] != rules.Reds || o.Groups[1] != rules.Yellows || !o.Continue {
		t.Fatalf("bad assignment %+v", o)
	}
}

func TestPocketingOwnColourContinuesWithoutCall(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 4, Pocketed: []int{4}, PocketByBall: map[int]int{4: 1}}
	out := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 4, 5, 8, 9), balls(0, 5, 8, 9), rep)
	if out.Foul || !out.Continue || out.NextPlayer != 0 {
		t.Fatalf("own red pot should continue without a call: %+v", out)
	}
}

func TestEarlyEightLoses(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 1, Pocketed: []int{8}, PocketByBall: map[int]int{8: 0}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 8, 9), balls(0, 1, 9), rep)
	if o.Winner != 1 {
		t.Fatalf("expected loss %+v", o)
	}
}

func TestLegalEightWins(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 8, Pocketed: []int{8}, PocketByBall: map[int]int{8: 4}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 8, 9), balls(0, 9), rep)
	if o.Winner != 0 {
		t.Fatalf("expected win %+v", o)
	}
}

func TestDryBreakWithContactPassesOpenTableWithoutRailRequirement(t *testing.T) {
	st := rules.State{Break: true, Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: 1, PocketByBall: map[int]int{}, RailBallIDs: []int{1, 2}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 2, 8), balls(0, 1, 2, 8), rep)
	if o.Foul || o.BreakChoice.Required || o.Continue || o.NextPlayer != 1 || o.Groups != [2]rules.Group{rules.Open, rules.Open} {
		t.Fatalf("dry break should pass the open table normally: %+v", o)
	}
}

func TestBreakWithoutObjectContactStillOffersFoulChoice(t *testing.T) {
	st := rules.State{Break: true, Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: -1, PocketByBall: map[int]int{}}
	out := rules.Evaluate(st, 0, rules.ShotCall{}, fullTable(), fullTable(), rep)
	if !out.Foul || out.FoulCode != "break_no_contact" || !out.BreakChoice.Required || out.BreakChoice.Actor != 1 {
		t.Fatalf("break without contact must remain a foul: %+v", out)
	}
}

func TestScratchGivesOpponentBallInHand(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 1, CueScratch: true, Pocketed: []int{0}, PocketByBall: map[int]int{0: 0}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 8, 9), balls(1, 8, 9), rep)
	if !o.Foul || o.FoulCode != "scratch" || !o.BallInHand || o.NextPlayer != 1 {
		t.Fatalf("scratch outcome incorrect %+v", o)
	}
}

func TestNoRailAfterContactIsFoul(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 1, PocketByBall: map[int]int{}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 8, 9), balls(0, 1, 8, 9), rep)
	if !o.Foul || o.FoulCode != "no_rail_after_contact" {
		t.Fatalf("expected no-rail foul %+v", o)
	}
}

func TestSafetyEndsTurnEvenWhenOwnColourFalls(t *testing.T) {
	st := rules.State{Groups: [2]rules.Group{rules.Reds, rules.Yellows}}
	rep := physics.ShotReport{FirstObjectBall: 2, Pocketed: []int{2}, PocketByBall: map[int]int{2: 3}}
	o := rules.Evaluate(st, 0, rules.ShotCall{Safety: true}, balls(0, 2, 8, 9), balls(0, 8, 9), rep)
	if o.Foul || o.Continue || o.NextPlayer != 1 {
		t.Fatalf("safety outcome incorrect %+v", o)
	}
}

func TestEightOnLegalBreakOffersBreakerChoice(t *testing.T) {
	st := rules.State{Break: true, Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: 1, Pocketed: []int{8}, PocketByBall: map[int]int{8: 2}, RailBallIDs: []int{1, 2, 3, 4}}
	o := rules.Evaluate(st, 0, rules.ShotCall{}, balls(0, 1, 2, 3, 4, 8), balls(0, 1, 2, 3, 4), rep)
	if !o.BreakChoice.Required || o.BreakChoice.Actor != 0 || o.BreakChoice.Kind != "eight_legal" {
		t.Fatalf("expected breaker choice %+v", o)
	}
}

func TestOpenTableEightFirstIsFoul(t *testing.T) {
	before := fullTable()
	rep := physics.ShotReport{FirstObjectBall: 8, PocketByBall: map[int]int{}, AnyRailAfterFirst: true}
	out := rules.Evaluate(rules.State{Groups: [2]rules.Group{rules.Open, rules.Open}}, 0, rules.ShotCall{}, before, before, rep)
	if !out.Foul || out.FoulCode != "wrong_ball_first" {
		t.Fatalf("expected open-table 8-first foul: %+v", out)
	}
}

func TestLegalDryBreakPassesOpenTableToOpponent(t *testing.T) {
	st := rules.State{Break: true, Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: 1, PocketByBall: map[int]int{}, RailBallIDs: []int{1, 2, 3, 4}}
	out := rules.Evaluate(st, 0, rules.ShotCall{}, fullTable(), fullTable(), rep)
	if out.Foul || out.Continue || out.NextPlayer != 1 || out.Groups != [2]rules.Group{rules.Open, rules.Open} || out.BreakChoice.Required {
		t.Fatalf("legal dry break must pass an open table to opponent: %+v", out)
	}
}

func TestBreakPotContinuesWithoutAssigningColours(t *testing.T) {
	st := rules.State{Break: true, Groups: [2]rules.Group{rules.Open, rules.Open}}
	rep := physics.ShotReport{FirstObjectBall: 1, Pocketed: []int{9}, PocketByBall: map[int]int{9: 2}}
	out := rules.Evaluate(st, 0, rules.ShotCall{}, fullTable(), balls(0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 15), rep)
	if out.Foul || !out.Continue || out.NextPlayer != 0 || out.Groups != [2]rules.Group{rules.Open, rules.Open} {
		t.Fatalf("break pot must continue with colours still open: %+v", out)
	}
}
