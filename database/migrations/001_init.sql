CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username varchar(32) NOT NULL UNIQUE,
  display_name varchar(32) NOT NULL,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS guest_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash char(64) NOT NULL UNIQUE,
  nickname varchar(24) NOT NULL,
  csrf_token varchar(96) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_guest_sessions_exp ON guest_sessions(expires_at);

CREATE TABLE IF NOT EXISTS auth_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash char(64) NOT NULL UNIQUE,
  csrf_token varchar(96) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_exp ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS lobbies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  short_code varchar(10) NOT NULL UNIQUE,
  creator_principal varchar(96) NOT NULL,
  name varchar(48) NOT NULL,
  visibility varchar(12) NOT NULL CHECK (visibility IN ('public','private')),
  password_hash text,
  shot_timer_seconds integer NOT NULL DEFAULT 45 CHECK (shot_timer_seconds IN (0,30,45,60)),
  ruleset_version varchar(32) NOT NULL DEFAULT 'wpa-8ball-v1',
  table_config_version varchar(32) NOT NULL DEFAULT 'wpa-9ft-v1',
  created_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_lobbies_open ON lobbies(closed_at, created_at DESC);

CREATE TABLE IF NOT EXISTS matches (
  id uuid PRIMARY KEY,
  lobby_id uuid NOT NULL REFERENCES lobbies(id),
  ruleset_version varchar(32) NOT NULL,
  physics_version varchar(32) NOT NULL,
  table_config_version varchar(32) NOT NULL,
  engine_version varchar(64) NOT NULL,
  rack_seed bigint NOT NULL,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  winner_principal varchar(96),
  loser_principal varchar(96),
  end_reason varchar(48),
  duration_ms bigint
);
CREATE INDEX IF NOT EXISTS idx_matches_lobby_time ON matches(lobby_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_matches_winner ON matches(winner_principal, started_at DESC);

CREATE TABLE IF NOT EXISTS match_players (
  match_id uuid NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  seat smallint NOT NULL CHECK (seat IN (0,1)),
  principal varchar(96) NOT NULL,
  user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  guest_id uuid REFERENCES guest_sessions(id) ON DELETE SET NULL,
  nickname varchar(24) NOT NULL,
  cue_skin varchar(32) NOT NULL,
  assigned_group varchar(12) NOT NULL DEFAULT 'open',
  fouls integer NOT NULL DEFAULT 0,
  balls_pocketed integer NOT NULL DEFAULT 0,
  result varchar(12),
  PRIMARY KEY(match_id, seat)
);
CREATE INDEX IF NOT EXISTS idx_match_players_principal ON match_players(principal, match_id);

CREATE TABLE IF NOT EXISTS shots (
  id bigserial PRIMARY KEY,
  match_id uuid NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  shot_number integer NOT NULL,
  principal varchar(96) NOT NULL,
  aim_angle double precision NOT NULL,
  power double precision NOT NULL,
  cue_offset_x double precision NOT NULL,
  cue_offset_y double precision NOT NULL,
  called_ball smallint,
  called_pocket smallint,
  safety boolean NOT NULL DEFAULT false,
  started_at timestamptz NOT NULL,
  simulation_duration_ms integer NOT NULL,
  foul_code varchar(48),
  final_state_hash char(64) NOT NULL,
  UNIQUE(match_id, shot_number)
);

CREATE TABLE IF NOT EXISTS player_statistics (
  principal varchar(96) PRIMARY KEY,
  matches_played integer NOT NULL DEFAULT 0,
  wins integer NOT NULL DEFAULT 0,
  losses integer NOT NULL DEFAULT 0,
  balls_pocketed integer NOT NULL DEFAULT 0,
  fouls integer NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS match_checkpoints (
  match_id uuid PRIMARY KEY REFERENCES matches(id) ON DELETE CASCADE,
  checkpoint_json jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
