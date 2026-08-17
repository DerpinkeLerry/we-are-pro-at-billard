package lobby

import (
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/persistence"
	"poolarena/game-server/internal/physics"
	"strings"
	"testing"
	"time"
)

func testLobby(t *testing.T) *Lobby {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	claims := auth.Claims{LobbyID: "lobby", LobbyCode: "ABC123", LobbyName: "Rotation Test"}
	return New(claims, cfg, physics.NewEngine(cfg.Table, cfg.Physics), persistence.Memory{})
}

func TestThreePlayerFIFORotation(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{}
	for _, id := range []string{"A", "B", "C"} {
		l.participants[id] = &Participant{Principal: id, PublicID: id, Nickname: id, Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}}
	}
	l.active = [2]string{"A", "B"}
	l.participants["A"].ActiveSeat = 0
	l.participants["B"].ActiveSeat = 1
	l.queue = []string{"C"}

	l.rotateActiveAfterMatch()
	if got := strings.Join(l.queue, ""); got != "CAB" {
		t.Fatalf("after A/B expected queue C,A,B; got %v", l.queue)
	}
	l.State = Rotating
	l.selectPlayersIfPossible()
	if l.active != [2]string{"C", "A"} || strings.Join(l.queue, "") != "B" {
		t.Fatalf("expected C vs A, B waiting; active=%v queue=%v", l.active, l.queue)
	}

	l.rotateActiveAfterMatch()
	if got := strings.Join(l.queue, ""); got != "BCA" {
		t.Fatalf("after C/A expected queue B,C,A; got %v", l.queue)
	}
	l.State = Rotating
	l.selectPlayersIfPossible()
	if l.active != [2]string{"B", "C"} || strings.Join(l.queue, "") != "A" {
		t.Fatalf("expected B vs C, A waiting; active=%v queue=%v", l.active, l.queue)
	}
}

func TestSpectatorDisconnectLeavesQueue(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"A": {Principal: "A", PublicID: "A", Connected: true, ActiveSeat: 0, RecentRequests: map[string]time.Time{}},
		"B": {Principal: "B", PublicID: "B", Connected: true, ActiveSeat: 1, RecentRequests: map[string]time.Time{}},
		"C": {Principal: "C", PublicID: "C", Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}},
	}
	l.active = [2]string{"A", "B"}
	l.queue = []string{"C"}
	l.removeFromQueue("C")
	delete(l.participants, "C")
	if len(l.queue) != 0 || l.participants["C"] != nil {
		t.Fatal("spectator remained in rotation")
	}
}

func TestLoneParticipantOwnsBothSoloSeats(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"A": {Principal: "A", PublicID: "A", Nickname: "A", Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}},
	}
	l.queue = []string{"A"}
	l.selectPlayersIfPossible()
	if l.active != [2]string{"A", "A"} || l.State != Starting || len(l.queue) != 0 {
		t.Fatalf("lone participant should receive both solo seats: active=%v state=%s queue=%v", l.active, l.State, l.queue)
	}
	if !l.participantOwnsSeat(l.participants["A"], 0) || !l.participantOwnsSeat(l.participants["A"], 1) {
		t.Fatal("solo participant does not own both seats")
	}
	state := l.publicState()
	if !state.Solo || len(state.Participants) != 1 || len(state.Participants[0].Seats) != 2 {
		t.Fatalf("solo ownership missing from public state: %+v", state)
	}
}

func TestRealOpponentReplacesUnstartedSoloSeat(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"A": {Principal: "A", PublicID: "A", Nickname: "A", Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}},
	}
	l.queue = []string{"A"}
	l.selectPlayersIfPossible()
	l.participants["A"].Ready = true
	l.countdownDeadline = time.Now().Add(time.Second)
	l.participants["B"] = &Participant{Principal: "B", PublicID: "B", Nickname: "B", Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}}
	l.queue = append(l.queue, "B")
	l.selectPlayersIfPossible()
	if l.active != [2]string{"A", "B"} || l.State != Starting || len(l.queue) != 0 {
		t.Fatalf("real opponent should replace solo seat: active=%v state=%s queue=%v", l.active, l.State, l.queue)
	}
	if l.participants["A"].Ready || l.participants["B"].Ready || !l.countdownDeadline.IsZero() {
		t.Fatal("opponent promotion must restart the ready phase")
	}
}

func TestSoloRotationQueuesPrincipalOnlyOnce(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"A": {Principal: "A", PublicID: "A", Nickname: "A", Connected: true, ActiveSeat: 0, RecentRequests: map[string]time.Time{}},
	}
	l.active = [2]string{"A", "A"}
	l.rotateActiveAfterMatch()
	if len(l.queue) != 1 || l.queue[0] != "A" || l.active != [2]string{} {
		t.Fatalf("solo rotation duplicated participant: active=%v queue=%v", l.active, l.queue)
	}
}

func TestRemainingPlayerFallsBackToSoloSeats(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"B": {Principal: "B", PublicID: "B", Nickname: "B", Connected: true, ActiveSeat: 1, RecentRequests: map[string]time.Time{}},
	}
	l.active = [2]string{"", "B"}
	l.selectPlayersIfPossible()
	if l.active != [2]string{"B", "B"} || l.participants["B"].ActiveSeat != 0 || l.State != Starting {
		t.Fatalf("remaining player did not fall back to solo: active=%v state=%s", l.active, l.State)
	}
}

func TestRemainingPlayerKeepsSeatWhenOpponentIsWaiting(t *testing.T) {
	l := testLobby(t)
	l.participants = map[string]*Participant{
		"B": {Principal: "B", PublicID: "B", Nickname: "B", Connected: true, ActiveSeat: 1, RecentRequests: map[string]time.Time{}},
		"C": {Principal: "C", PublicID: "C", Nickname: "C", Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}},
	}
	l.active = [2]string{"", "B"}
	l.queue = []string{"C"}
	l.selectPlayersIfPossible()
	if l.active != [2]string{"C", "B"} || l.participants["C"].ActiveSeat != 0 || l.participants["B"].ActiveSeat != 1 || l.State != Starting {
		t.Fatalf("waiting opponent was not seated correctly: active=%v state=%s", l.active, l.State)
	}
}
