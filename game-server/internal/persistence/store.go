package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	_ "github.com/lib/pq"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/rules"
	"time"
)

type Store interface {
	Ping(context.Context) error
	BeginMatch(context.Context, *match.Game, string, string, string) error
	RecordShot(context.Context, string, int, string, match.ShotInput, physics.Simulation, rules.Outcome) error
	FinishMatch(context.Context, *match.Game) error
	SaveCheckpoint(context.Context, string, any) error
	CloseLobby(context.Context, string) error
	Close() error
}

type Postgres struct{ db *sql.DB }

func Open(url string) (*Postgres, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &Postgres{db: db}, nil
}
func (p *Postgres) Ping(ctx context.Context) error { return p.db.PingContext(ctx) }
func (p *Postgres) Close() error                   { return p.db.Close() }

func (p *Postgres) BeginMatch(ctx context.Context, g *match.Game, ruleset, physicsVersion, tableVersion string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO matches(id,lobby_id,ruleset_version,physics_version,table_config_version,engine_version,rack_seed,started_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, g.ID, g.LobbyID, ruleset, physicsVersion, tableVersion, "pool-arena-go-v1", g.RackSeed, g.StartedAt)
	if err != nil {
		return err
	}
	for i, pl := range g.Players {
		var userID, guestID any
		if len(pl.Principal) > 5 && pl.Principal[:5] == "user:" {
			userID = pl.Principal[5:]
		} else if len(pl.Principal) > 6 && pl.Principal[:6] == "guest:" {
			guestID = pl.Principal[6:]
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO match_players(match_id,seat,principal,user_id,guest_id,nickname,cue_skin) VALUES($1,$2,$3,$4,$5,$6,$7)`, g.ID, i, pl.Principal, userID, guestID, pl.Nickname, pl.CueSkin)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *Postgres) RecordShot(ctx context.Context, matchID string, shotNumber int, principal string, in match.ShotInput, sim physics.Simulation, out rules.Outcome) error {
	raw, _ := json.Marshal(physics.SnapshotBalls(sim.Final))
	sum := sha256.Sum256(raw)
	var cb, cp any
	if in.CalledBall > 0 {
		cb = in.CalledBall
	}
	if in.CalledPocket >= 0 {
		cp = in.CalledPocket
	}
	_, err := p.db.ExecContext(ctx, `INSERT INTO shots(match_id,shot_number,principal,aim_angle,power,cue_offset_x,cue_offset_y,called_ball,called_pocket,safety,started_at,simulation_duration_ms,foul_code,final_state_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(match_id,shot_number) DO NOTHING`, matchID, shotNumber, principal, in.AimAngle, in.Power, in.CueOffsetX, in.CueOffsetY, cb, cp, in.Safety, time.Now().UTC(), int(sim.Report.SimulationDuration*1000), nullIfEmpty(out.FoulCode), hex.EncodeToString(sum[:]))
	return err
}

func (p *Postgres) FinishMatch(ctx context.Context, g *match.Game) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	duration := g.FinishedAt.Sub(g.StartedAt).Milliseconds()
	winner := ""
	loser := ""
	if g.Winner >= 0 {
		winner = g.Players[g.Winner].Principal
	}
	if g.Loser >= 0 {
		loser = g.Players[g.Loser].Principal
	}
	_, err = tx.ExecContext(ctx, `UPDATE matches SET finished_at=$2,winner_principal=$3,loser_principal=$4,end_reason=$5,duration_ms=$6 WHERE id=$1`, g.ID, g.FinishedAt, winner, loser, g.EndReason, duration)
	if err != nil {
		return err
	}
	for i, pl := range g.Players {
		result := "loss"
		win := 0
		loss := 1
		if i == g.Winner {
			result = "win"
			win = 1
			loss = 0
		}
		_, err = tx.ExecContext(ctx, `UPDATE match_players SET assigned_group=$3,fouls=$4,balls_pocketed=$5,result=$6 WHERE match_id=$1 AND seat=$2`, g.ID, i, string(g.RuleState.Groups[i]), g.Fouls[i], g.Pocketed[i], result)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO player_statistics(principal,matches_played,wins,losses,balls_pocketed,fouls) VALUES($1,1,$2,$3,$4,$5) ON CONFLICT(principal) DO UPDATE SET matches_played=player_statistics.matches_played+1,wins=player_statistics.wins+EXCLUDED.wins,losses=player_statistics.losses+EXCLUDED.losses,balls_pocketed=player_statistics.balls_pocketed+EXCLUDED.balls_pocketed,fouls=player_statistics.fouls+EXCLUDED.fouls,updated_at=now()`, pl.Principal, win, loss, g.Pocketed[i], g.Fouls[i])
		if err != nil {
			return err
		}
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM match_checkpoints WHERE match_id=$1`, g.ID)
	return tx.Commit()
}
func (p *Postgres) SaveCheckpoint(ctx context.Context, matchID string, state any) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `INSERT INTO match_checkpoints(match_id,checkpoint_json,updated_at) VALUES($1,$2,now()) ON CONFLICT(match_id) DO UPDATE SET checkpoint_json=EXCLUDED.checkpoint_json,updated_at=now()`, matchID, b)
	return err
}
func (p *Postgres) CloseLobby(ctx context.Context, lobbyID string) error {
	_, err := p.db.ExecContext(ctx, `UPDATE lobbies SET closed_at=COALESCE(closed_at,now()) WHERE id=$1`, lobbyID)
	return err
}
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
