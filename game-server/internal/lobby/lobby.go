package lobby

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"math"
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/config"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/persistence"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/protocol"
	"poolarena/game-server/internal/realtime"
	"poolarena/game-server/internal/rules"
	"strings"
	"time"
	"unicode"
)

type Lobby struct {
	ID, Code, Name                                                   string
	State                                                            string
	ShotTimer                                                        int
	cfg                                                              config.All
	engine                                                           *physics.Engine
	store                                                            persistence.Store
	participants                                                     map[string]*Participant
	queue                                                            []string
	active                                                           [2]string
	game                                                             *match.Game
	soloMatch                                                        bool
	join                                                             chan joinCmd
	leave                                                            chan leaveCmd
	messages                                                         chan msgCmd
	shutdown                                                         chan shutdownCmd
	summary                                                          chan summaryCmd
	readyDeadline, countdownDeadline, postGameDeadline, shotDeadline time.Time
	playback                                                         *Playback
	pausedGameState                                                  string
	pauseStarted                                                     time.Time
	emptySince                                                       time.Time
}

func New(c auth.Claims, cfg config.All, engine *physics.Engine, store persistence.Store) *Lobby {
	return &Lobby{ID: c.LobbyID, Code: c.LobbyCode, Name: c.LobbyName, State: Waiting, ShotTimer: c.ShotTimerSeconds, cfg: cfg, engine: engine, store: store, participants: map[string]*Participant{}, join: make(chan joinCmd, 32), leave: make(chan leaveCmd, 32), messages: make(chan msgCmd, 128), shutdown: make(chan shutdownCmd), summary: make(chan summaryCmd)}
}

func (l *Lobby) Run() {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case c := <-l.join:
			l.handleJoin(c)
		case c := <-l.leave:
			l.handleLeave(c)
		case c := <-l.messages:
			l.handleMessage(c)
		case c := <-l.summary:
			c.reply <- l.runtimeSummary()
		case <-ticker.C:
			if l.tick() {
				return
			}
		case c := <-l.shutdown:
			l.gracefulShutdown()
			close(c.done)
			return
		}
	}
}

func (l *Lobby) handleJoin(c joinCmd) {
	p := l.participants[c.claims.Sub]
	if p != nil {
		if p.Conn != nil && p.Conn != c.client {
			_ = p.Conn.Conn.Close()
		}
		p.Conn = c.client
		p.Connected = true
		p.ReconnectDeadline = time.Time{}
		p.Nickname = c.claims.Nickname
		p.CueSkin = c.claims.CueSkin
		c.client.SendEvent("AUTH_OK", map[string]any{"participantId": p.PublicID, "reconnected": true})
		if l.game != nil && l.game.State == match.StatePaused && l.allActiveConnected() {
			pauseDur := time.Since(l.pauseStarted)
			if l.playback != nil && l.playback.Paused {
				l.playback.Start = l.playback.Start.Add(pauseDur)
				l.playback.Paused = false
			}
			l.game.Resume(l.pausedGameState)
			l.setShotDeadline()
			l.broadcast("PLAYER_RECONNECTED", map[string]any{"participantId": p.PublicID})
		}
		slog.Info("lobby reconnect", "lobby", l.Code, "participant", p.PublicID)
		l.sendFullState(c.client)
		l.broadcastState()
		return
	}
	p = &Participant{Principal: c.claims.Sub, PublicID: randomID(), Nickname: c.claims.Nickname, CueSkin: c.claims.CueSkin, Conn: c.client, Connected: true, ActiveSeat: -1, RecentRequests: map[string]time.Time{}}
	l.participants[p.Principal] = p
	l.queue = append(l.queue, p.Principal)
	l.emptySince = time.Time{}
	c.client.SendEvent("AUTH_OK", map[string]any{"participantId": p.PublicID, "reconnected": false})
	slog.Info("lobby join", "lobby", l.Code, "participant", p.PublicID)
	l.selectPlayersIfPossible()
	l.sendFullState(c.client)
	l.broadcastState()
}

func (l *Lobby) handleLeave(c leaveCmd) {
	p := l.participants[c.client.Principal]
	if p == nil || p.Conn != c.client {
		return
	}
	p.Connected = false
	p.Conn = nil
	if p.ActiveSeat >= 0 {
		p.ReconnectDeadline = time.Now().Add(time.Duration(l.cfg.Rules.ReconnectGraceSeconds) * time.Second)
		if l.game != nil && l.game.State != match.StateFinished && l.game.State != match.StatePaused {
			l.pausedGameState = l.game.State
			l.pauseStarted = time.Now()
			l.game.Pause()
			if l.playback != nil {
				l.playback.Paused = true
				l.playback.PauseStart = time.Now()
			}
			l.shotDeadline = time.Time{}
		}
		l.broadcast("PLAYER_RECONNECTING", map[string]any{"participantId": p.PublicID, "deadline": p.ReconnectDeadline.UnixMilli()})
	} else {
		l.removeFromQueue(p.Principal)
		delete(l.participants, p.Principal)
	}
	slog.Info("lobby disconnect", "lobby", l.Code, "participant", p.PublicID)
	l.broadcastState()
}

func (l *Lobby) handleMessage(c msgCmd) {
	p := l.participants[c.client.Principal]
	if p == nil || p.Conn != c.client || !p.Connected {
		return
	}
	m, ok := c.msg.(protocol.ClientMessage)
	if !ok {
		return
	}
	if !allowWindow(&p.MessageTimes, 120, 10*time.Second) {
		p.Conn.SendEvent("SERVER_ERROR", map[string]any{"code": "message_rate_limited"})
		_ = p.Conn.Conn.Close()
		return
	}
	switch m.Type {
	case "READY_SET":
		l.handleReady(p, m.Ready)
	case "SHOT_REQUEST":
		l.handleShot(p, m)
	case "BALL_IN_HAND_PLACE":
		l.handlePlacement(p, m)
	case "CHAT_SEND":
		l.handleChat(p, m.Text)
	case "CLIENT_PING":
		p.Conn.SendEvent("PONG", map[string]any{"clientTime": m.ClientTime})
	case "BREAK_OPTION":
		l.handleBreakChoice(p, m.Choice)
	case "LEAVE":
		_ = c.client.Conn.Close()
	}
}

func (l *Lobby) handleReady(p *Participant, ready bool) {
	if l.State != Starting || !l.participantIsActive(p) {
		return
	}
	p.Ready = ready
	l.broadcast("READY_STATE", l.publicState())
	if l.activeReady() {
		l.countdownDeadline = time.Now().Add(time.Duration(l.cfg.Rules.CountdownSeconds) * time.Second)
		l.broadcast("COUNTDOWN", map[string]any{"endsAt": l.countdownDeadline.UnixMilli()})
	}
}

func (l *Lobby) handleShot(p *Participant, m protocol.ClientMessage) {
	if !l.participantIsActive(p) {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "spectator_cannot_shoot"})
		return
	}
	if l.game == nil {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "no_active_match"})
		return
	}
	shooter := l.game.Turn
	if !l.participantOwnsSeat(p, shooter) {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "not_your_turn"})
		return
	}
	if !allowWindow(&p.ShotTimes, 8, 2*time.Second) {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "shot_rate_limited"})
		return
	}
	if m.MatchID != l.game.ID {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "wrong_match"})
		return
	}
	if m.RequestID == "" || l.requestSeen(p, m.RequestID) {
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": "duplicate_request"})
		return
	}
	in := match.ShotInput{RequestID: m.RequestID, TurnNonce: m.TurnNonce, AimAngle: m.AimAngle, Power: m.Power, CueOffsetX: m.CueOffsetX, CueOffsetY: m.CueOffsetY, CalledBall: m.CalledBall, CalledPocket: m.CalledPocket, Safety: m.Safety}
	sim, out, err := l.game.StartShot(shooter, in)
	if err != nil {
		slog.Warn("shot rejected", "lobby", l.Code, "participant", p.PublicID, "reason", err.Error())
		p.Conn.SendEvent("SHOT_REJECTED", map[string]any{"requestId": m.RequestID, "reason": err.Error()})
		return
	}
	l.shotDeadline = time.Time{}
	l.playback = &Playback{Start: time.Now(), SimDuration: time.Duration(sim.Report.SimulationDuration * float64(time.Second))}
	l.broadcast("SHOT_ACCEPTED", map[string]any{"requestId": m.RequestID, "shooter": shooter, "durationMs": int64(sim.Report.SimulationDuration * 1000)})
	l.broadcast("COLLISION_EVENTS", sim.Report.Events)
	matchID := l.game.ID
	shotNo := l.game.ShotNumber + 1
	principal := p.Principal
	if !l.soloMatch {
		go l.persistShot(matchID, shotNo, principal, in, sim, out)
	}
}

func (l *Lobby) handlePlacement(p *Participant, m protocol.ClientMessage) {
	if l.game == nil || !l.participantOwnsSeat(p, l.game.Turn) {
		return
	}
	if err := l.game.PlaceCueBall(l.game.Turn, physics.Vec2{X: m.X, Y: m.Y}); err != nil {
		p.Conn.SendEvent("SERVER_ERROR", map[string]any{"code": "invalid_ball_in_hand", "message": err.Error()})
		return
	}
	l.setShotDeadline()
	l.broadcast("MATCH_KEYFRAME", l.game.Public())
	l.broadcastState()
	l.saveCheckpoint()
}
func (l *Lobby) handleBreakChoice(p *Participant, choice string) {
	if l.game == nil || !l.participantOwnsSeat(p, l.game.PendingChoice.Actor) {
		return
	}
	if err := l.game.ResolveBreakChoice(l.game.PendingChoice.Actor, choice); err != nil {
		p.Conn.SendEvent("SERVER_ERROR", map[string]any{"code": "invalid_break_choice", "message": err.Error()})
		return
	}
	l.setShotDeadline()
	l.broadcast("MATCH_KEYFRAME", l.game.Public())
	l.broadcastState()
	l.saveCheckpoint()
}

func (l *Lobby) handleChat(p *Participant, text string) {
	now := time.Now()
	kept := p.ChatTimes[:0]
	for _, x := range p.ChatTimes {
		if now.Sub(x) < 10*time.Second {
			kept = append(kept, x)
		}
	}
	p.ChatTimes = kept
	if len(p.ChatTimes) >= 5 {
		p.Conn.SendEvent("SERVER_ERROR", map[string]any{"code": "chat_rate_limited"})
		return
	}
	text = sanitizeChat(text)
	if text == "" {
		return
	}
	p.ChatTimes = append(p.ChatTimes, now)
	l.broadcast("CHAT_MESSAGE", map[string]any{"participantId": p.PublicID, "nickname": p.Nickname, "text": text, "timestamp": now.UnixMilli()})
}

func (l *Lobby) tick() bool {
	now := time.Now()
	for _, p := range l.participants {
		if p.ActiveSeat >= 0 && !p.Connected && !p.ReconnectDeadline.IsZero() && now.After(p.ReconnectDeadline) {
			seat := p.ActiveSeat
			if l.game != nil && l.game.State != match.StateFinished {
				l.game.Forfeit(seat, "disconnect_forfeit")
				l.handleGameFinished()
				l.broadcastState()
				continue
			}
			for i := range l.active {
				if l.active[i] == p.Principal {
					l.active[i] = ""
				}
			}
			delete(l.participants, p.Principal)
			p.ActiveSeat = -1
			l.selectPlayersIfPossible()
			l.broadcastState()
		}
	}
	if l.playback != nil && !l.playback.Paused && l.game != nil && l.game.Pending != nil {
		elapsed := time.Since(l.playback.Start)
		frames := l.game.Pending.Simulation.Frames
		latest := -1
		for l.playback.NextFrame < len(frames) && time.Duration(frames[l.playback.NextFrame].Time*float64(time.Second)) <= elapsed {
			latest = l.playback.NextFrame
			l.playback.NextFrame++
		}
		if latest >= 0 {
			l.broadcastSnapshot(l.game.ID, frames[latest].Time, frames[latest].Balls)
		}
		if elapsed >= l.playback.SimDuration {
			l.finishPlayback()
		}
	}
	if l.State == Starting && !l.readyDeadline.IsZero() && now.After(l.readyDeadline) && l.countdownDeadline.IsZero() {
		l.readyTimeout()
	}
	if l.State == Starting && !l.countdownDeadline.IsZero() && now.After(l.countdownDeadline) {
		l.startGame()
	}
	if !l.shotDeadline.IsZero() && now.After(l.shotDeadline) && l.game != nil && (l.game.State == match.StateAwaitingShot || l.game.State == match.StateBallInHand) {
		loser := l.game.Turn
		l.game.ShotClockFoul()
		l.broadcast("FOUL", map[string]any{"player": loser, "code": "shot_clock"})
		l.setShotDeadline()
		l.broadcastState()
		l.saveCheckpoint()
	}
	if l.State == PostGame && !l.postGameDeadline.IsZero() && now.After(l.postGameDeadline) {
		l.game = nil
		l.soloMatch = false
		l.State = Rotating
		l.selectPlayersIfPossible()
		l.broadcast("NEXT_MATCH", l.publicState())
		l.broadcastState()
	}
	if len(l.participants) == 0 {
		if l.emptySince.IsZero() {
			l.emptySince = now
		}
		if now.Sub(l.emptySince) > time.Duration(l.cfg.Rules.EmptyLobbySeconds)*time.Second {
			l.State = Closing
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if err := l.store.CloseLobby(ctx, l.ID); err != nil {
				slog.Error("close empty lobby persistence", "lobby", l.Code, "error", err)
			}
			cancel()
			slog.Info("lobby closing empty", "lobby", l.Code)
			return true
		}
	} else {
		l.emptySince = time.Time{}
	}
	return false
}

func (l *Lobby) finishPlayback() {
	if l.game == nil || l.game.Pending == nil {
		return
	}
	out, err := l.game.FinishShot()
	if err != nil {
		slog.Error("finish shot", "error", err)
		return
	}
	l.playback = nil
	l.broadcast("SHOT_RESOLVED", map[string]any{"outcome": out, "match": l.game.Public()})
	if out.Foul {
		l.broadcast("FOUL", map[string]any{"player": 1 - out.NextPlayer, "code": out.FoulCode})
	}
	if out.BreakChoice.Required {
		l.broadcast("BREAK_OPTION_REQUIRED", out.BreakChoice)
	}
	if l.game.State == match.StateFinished {
		l.handleGameFinished()
		return
	}
	l.setShotDeadline()
	l.broadcastState()
	l.saveCheckpoint()
}

func (l *Lobby) handleGameFinished() {
	if l.game == nil {
		return
	}
	g := l.game
	l.State = PostGame
	l.shotDeadline = time.Time{}
	l.playback = nil
	l.postGameDeadline = time.Now().Add(time.Duration(l.cfg.Rules.PostGameSeconds) * time.Second)
	l.broadcast("MATCH_FINISHED", map[string]any{"match": g.Public(), "nextMatchAt": l.postGameDeadline.UnixMilli()})
	slog.Info("match result", "lobby", l.Code, "match", g.ID, "winnerSeat", g.Winner, "reason", g.EndReason, "durationMs", g.FinishedAt.Sub(g.StartedAt).Milliseconds())
	if !l.soloMatch {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			if err := l.store.FinishMatch(ctx, g); err != nil {
				slog.Error("persist match finish", "error", err)
			}
		}()
	}
	l.rotateActiveAfterMatch()
	l.broadcastState()
}

func (l *Lobby) selectPlayersIfPossible() {
	if l.State == PostGame {
		return
	}
	// If a real opponent arrives while a solo practice match is still waiting
	// for Ready, give that opponent seat 1 and cancel the solo countdown.
	if l.game == nil && l.active[0] != "" && l.active[0] == l.active[1] {
		if opponent := l.popNextConnected(l.active[0]); opponent != "" {
			l.active[1] = opponent
			l.participants[opponent].ActiveSeat = 1
			if p := l.participants[l.active[0]]; p != nil {
				p.ActiveSeat = 0
				p.Ready = false
			}
			l.participants[opponent].Ready = false
			l.State = Starting
			l.readyDeadline = time.Now().Add(time.Duration(l.cfg.Rules.ReadyTimeoutSeconds) * time.Second)
			l.countdownDeadline = time.Time{}
			l.broadcastState()
			return
		}
	}
	if l.game == nil && (l.active[0] == "") != (l.active[1] == "") {
		occupiedSeat := 0
		emptySeat := 1
		if l.active[0] == "" {
			occupiedSeat, emptySeat = 1, 0
		}
		incumbent := l.active[occupiedSeat]
		if opponent := l.popNextConnected(incumbent); opponent != "" {
			l.active[emptySeat] = opponent
			l.participants[opponent].ActiveSeat = emptySeat
			l.participants[opponent].Ready = false
			l.participants[incumbent].ActiveSeat = occupiedSeat
		} else {
			l.active[emptySeat] = incumbent
			l.participants[incumbent].ActiveSeat = 0
		}
		if p := l.participants[incumbent]; p != nil {
			p.Ready = false
		}
		l.State = Starting
		l.readyDeadline = time.Now().Add(time.Duration(l.cfg.Rules.ReadyTimeoutSeconds) * time.Second)
		l.countdownDeadline = time.Time{}
		l.broadcastState()
		return
	}

	if l.active[0] == "" && l.active[1] == "" {
		first := l.popNextConnected("")
		if first == "" {
			l.State = Waiting
			l.readyDeadline = time.Time{}
			l.countdownDeadline = time.Time{}
			l.broadcastState()
			return
		}
		l.active[0] = first
		l.participants[first].ActiveSeat = 0
		l.participants[first].Ready = false
		second := l.popNextConnected(first)
		if second == "" {
			// Solo practice: the same authenticated participant owns both seats.
			l.active[1] = first
		} else {
			l.active[1] = second
			l.participants[second].ActiveSeat = 1
			l.participants[second].Ready = false
		}
	}
	for i := 0; i < 2; i++ {
		if l.active[i] != "" {
			continue
		}
		for len(l.queue) > 0 {
			pid := l.queue[0]
			l.queue = l.queue[1:]
			p := l.participants[pid]
			if p == nil || !p.Connected {
				continue
			}
			l.active[i] = pid
			p.ActiveSeat = i
			p.Ready = false
			break
		}
	}
	if l.active[0] != "" && l.active[1] != "" {
		l.State = Starting
		l.readyDeadline = time.Now().Add(time.Duration(l.cfg.Rules.ReadyTimeoutSeconds) * time.Second)
		l.countdownDeadline = time.Time{}
	} else {
		l.State = Waiting
	}
	l.broadcastState()
}

func (l *Lobby) popNextConnected(exclude string) string {
	for len(l.queue) > 0 {
		pid := l.queue[0]
		l.queue = l.queue[1:]
		if pid == exclude {
			continue
		}
		if p := l.participants[pid]; p != nil && p.Connected {
			return pid
		}
	}
	return ""
}

func (l *Lobby) seatsFor(principal string) []int {
	seats := []int{}
	for seat, pid := range l.active {
		if pid == principal {
			seats = append(seats, seat)
		}
	}
	return seats
}

func (l *Lobby) participantIsActive(p *Participant) bool {
	return p != nil && len(l.seatsFor(p.Principal)) > 0
}

func (l *Lobby) participantOwnsSeat(p *Participant, seat int) bool {
	return p != nil && seat >= 0 && seat < len(l.active) && l.active[seat] == p.Principal
}

func (l *Lobby) isSoloActive() bool {
	return l.active[0] != "" && l.active[0] == l.active[1]
}

func (l *Lobby) rotateActiveAfterMatch() {
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		pid := l.active[i]
		l.active[i] = ""
		if pid == "" || seen[pid] {
			continue
		}
		seen[pid] = true
		if p := l.participants[pid]; p != nil {
			p.ActiveSeat = -1
			p.Ready = false
			if p.Connected {
				l.queue = append(l.queue, pid)
			} else {
				delete(l.participants, pid)
			}
		}
	}
}
func (l *Lobby) readyTimeout() {
	front := []string{}
	back := []string{}
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		pid := l.active[i]
		l.active[i] = ""
		if pid == "" || seen[pid] {
			continue
		}
		seen[pid] = true
		if p := l.participants[pid]; p != nil {
			p.ActiveSeat = -1
			if p.Connected {
				if p.Ready {
					front = append(front, pid)
				} else {
					back = append(back, pid)
				}
			}
		}
	}
	l.queue = append(front, l.queue...)
	l.queue = append(l.queue, back...)
	l.State = Rotating
	l.selectPlayersIfPossible()
}
func (l *Lobby) startGame() {
	if !l.activeReady() || !l.allActiveConnected() {
		l.countdownDeadline = time.Time{}
		return
	}
	players := [2]match.Player{}
	for i := 0; i < 2; i++ {
		p := l.participants[l.active[i]]
		players[i] = match.Player{Principal: p.Principal, Nickname: p.Nickname, CueSkin: p.CueSkin}
	}
	l.game = match.New(l.ID, players, 0, l.cfg, l.engine)
	l.soloMatch = l.active[0] != "" && l.active[0] == l.active[1]
	l.State = Playing
	l.readyDeadline = time.Time{}
	l.countdownDeadline = time.Time{}
	l.broadcast("MATCH_STARTED", l.game.Public())
	slog.Info("match start", "lobby", l.Code, "match", l.game.ID)
	l.setShotDeadline()
	if !l.soloMatch {
		g := l.game
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			if err := l.store.BeginMatch(ctx, g, l.cfg.Rules.Version, l.cfg.Physics.Version, l.cfg.Table.Version); err != nil {
				slog.Error("persist match start", "error", err)
			}
		}()
	}
	l.broadcastState()
}

func (l *Lobby) setShotDeadline() {
	if l.ShotTimer <= 0 || l.game == nil || l.game.State == match.StateBreakOption || l.game.State == match.StateBallsMoving || l.game.State == match.StatePaused {
		l.shotDeadline = time.Time{}
		return
	}
	l.shotDeadline = time.Now().Add(time.Duration(l.ShotTimer) * time.Second)
	l.broadcast("TURN_STARTED", map[string]any{"turn": l.game.Turn, "turnNonce": l.game.TurnNonce, "deadline": l.shotDeadline.UnixMilli(), "ballInHand": l.game.BallInHand})
}
func (l *Lobby) activeReady() bool {
	for i := 0; i < 2; i++ {
		p := l.participants[l.active[i]]
		if p == nil || !p.Connected || !p.Ready {
			return false
		}
	}
	return true
}
func (l *Lobby) allActiveConnected() bool {
	for i := 0; i < 2; i++ {
		if l.active[i] == "" {
			continue
		}
		p := l.participants[l.active[i]]
		if p == nil || !p.Connected {
			return false
		}
	}
	return true
}

func (l *Lobby) publicState() PublicState {
	ps := PublicState{ID: l.ID, Code: l.Code, Name: l.Name, State: l.State, Solo: l.isSoloActive() || (l.soloMatch && l.game != nil)}
	if !l.shotDeadline.IsZero() {
		ps.ShotDeadline = l.shotDeadline.UnixMilli()
	}
	for _, p := range l.participants {
		role := "spectator"
		seat := -1
		seats := l.seatsFor(p.Principal)
		qp := l.queuePosition(p.Principal)
		if len(seats) > 0 {
			role = "player"
			seat = seats[0]
			qp = 0
		}
		ps.Participants = append(ps.Participants, PublicParticipant{ID: p.PublicID, Nickname: p.Nickname, CueSkin: p.CueSkin, Role: role, Seat: seat, Seats: seats, Ready: p.Ready, Reconnecting: !p.Connected, QueuePosition: qp})
	}
	for _, pid := range l.queue {
		if p := l.participants[pid]; p != nil {
			ps.Queue = append(ps.Queue, p.PublicID)
		}
	}
	if l.game != nil {
		m := l.game.Public()
		ps.Match = &m
	}
	return ps
}
func (l *Lobby) broadcastState() { l.broadcast("LOBBY_STATE", l.publicState()) }
func (l *Lobby) sendFullState(c *realtime.Client) {
	c.SendEvent("LOBBY_STATE", l.publicState())
	if l.game != nil {
		c.SendEvent("MATCH_KEYFRAME", l.game.Public())
	}
}
func (l *Lobby) broadcast(kind string, data any) {
	for _, p := range l.participants {
		if p.Connected && p.Conn != nil {
			if !p.Conn.SendEvent(kind, data) {
				_ = p.Conn.Conn.Close()
			}
		}
	}
}
func (l *Lobby) broadcastSnapshot(matchID string, simTime float64, balls []physics.BallSnapshot) {
	for _, p := range l.participants {
		if p.Connected && p.Conn != nil {
			p.Conn.SendPhysicsSnapshot(matchID, simTime, balls)
		}
	}
}
func (l *Lobby) runtimeSummary() RuntimeSummary {
	players := 0
	seen := map[string]bool{}
	for _, id := range l.active {
		if id != "" && !seen[id] {
			if p := l.participants[id]; p != nil && p.Connected {
				players++
				seen[id] = true
			}
		}
	}
	spec := 0
	for _, p := range l.participants {
		if p.Connected && !l.participantIsActive(p) {
			spec++
		}
	}
	return RuntimeSummary{ID: l.ID, Code: l.Code, Name: l.Name, State: l.State, Players: players, Spectators: spec, QueueSize: len(l.queue)}
}
func (l *Lobby) queuePosition(pid string) int {
	for i, x := range l.queue {
		if x == pid {
			return i + 1
		}
	}
	return 0
}
func (l *Lobby) removeFromQueue(pid string) {
	out := l.queue[:0]
	for _, x := range l.queue {
		if x != pid {
			out = append(out, x)
		}
	}
	l.queue = out
}
func (l *Lobby) requestSeen(p *Participant, id string) bool {
	now := time.Now()
	for k, t := range p.RecentRequests {
		if now.Sub(t) > 2*time.Minute {
			delete(p.RecentRequests, k)
		}
	}
	if _, ok := p.RecentRequests[id]; ok {
		return true
	}
	p.RecentRequests[id] = now
	return false
}
func (l *Lobby) persistShot(matchID string, shotNo int, principal string, in match.ShotInput, sim physics.Simulation, out rules.Outcome) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := l.store.RecordShot(ctx, matchID, shotNo, principal, in, sim, out); err != nil {
		slog.Error("persist shot", "error", err)
	}
}
func (l *Lobby) saveCheckpoint() {
	if l.game == nil || l.soloMatch {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	state := l.game.Public()
	if err := l.store.SaveCheckpoint(ctx, l.game.ID, state); err != nil {
		slog.Error("checkpoint", "error", err)
	}
}
func (l *Lobby) gracefulShutdown() {
	if l.game != nil && !l.soloMatch && l.game.State != match.StateFinished {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = l.store.SaveCheckpoint(ctx, l.game.ID, l.game.Public())
		cancel()
	}
	l.broadcast("SERVER_ERROR", map[string]any{"code": "server_restarting"})
	for _, p := range l.participants {
		if p.Conn != nil {
			_ = p.Conn.Conn.Close()
		}
	}
}

func allowWindow(times *[]time.Time, max int, window time.Duration) bool {
	now := time.Now()
	kept := (*times)[:0]
	for _, t := range *times {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	*times = kept
	if len(*times) >= max {
		return false
	}
	*times = append(*times, now)
	return true
}

func sanitizeChat(s string) string {
	s = strings.TrimSpace(s)
	r := []rune{}
	for _, x := range []rune(s) {
		if unicode.IsControl(x) && x != '\n' && x != '\t' {
			continue
		}
		r = append(r, x)
		if len(r) >= 300 {
			break
		}
	}
	return strings.TrimSpace(string(r))
}
func randomID() string              { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
