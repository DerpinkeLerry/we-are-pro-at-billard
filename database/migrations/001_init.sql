PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS guest_sessions (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  nickname TEXT NOT NULL,
  csrf_token TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_guest_sessions_exp ON guest_sessions(expires_at);

CREATE TABLE IF NOT EXISTS auth_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  csrf_token TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_exp ON auth_sessions(expires_at);

CREATE TABLE IF NOT EXISTS lobbies (
  id TEXT PRIMARY KEY,
  short_code TEXT NOT NULL UNIQUE,
  creator_principal TEXT NOT NULL,
  name TEXT NOT NULL,
  visibility TEXT NOT NULL CHECK (visibility IN ('public','private')),
  password_hash TEXT,
  shot_timer_seconds INTEGER NOT NULL DEFAULT 45 CHECK (shot_timer_seconds IN (0,30,45,60)),
  ruleset_version TEXT NOT NULL DEFAULT 'wpa-8ball-v1',
  table_config_version TEXT NOT NULL DEFAULT 'pool-7ft-v2',
  created_at INTEGER NOT NULL,
  closed_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_lobbies_open ON lobbies(closed_at, created_at DESC);

CREATE TABLE IF NOT EXISTS matches (
  id TEXT PRIMARY KEY,
  lobby_id TEXT NOT NULL REFERENCES lobbies(id),
  ruleset_version TEXT NOT NULL,
  physics_version TEXT NOT NULL,
  table_config_version TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  rack_seed INTEGER NOT NULL,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  winner_principal TEXT,
  loser_principal TEXT,
  end_reason TEXT,
  duration_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_matches_lobby_time ON matches(lobby_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_matches_winner ON matches(winner_principal, started_at DESC);

CREATE TABLE IF NOT EXISTS match_players (
  match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  seat INTEGER NOT NULL CHECK (seat IN (0,1)),
  principal TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
  guest_id TEXT REFERENCES guest_sessions(id) ON DELETE SET NULL,
  nickname TEXT NOT NULL,
  cue_skin TEXT NOT NULL,
  assigned_group TEXT NOT NULL DEFAULT 'open',
  fouls INTEGER NOT NULL DEFAULT 0,
  balls_pocketed INTEGER NOT NULL DEFAULT 0,
  result TEXT,
  PRIMARY KEY(match_id, seat)
);
CREATE INDEX IF NOT EXISTS idx_match_players_principal ON match_players(principal, match_id);

CREATE TABLE IF NOT EXISTS shots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  match_id TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
  shot_number INTEGER NOT NULL,
  principal TEXT NOT NULL,
  aim_angle REAL NOT NULL,
  power REAL NOT NULL,
  cue_offset_x REAL NOT NULL,
  cue_offset_y REAL NOT NULL,
  called_ball INTEGER,
  called_pocket INTEGER,
  safety INTEGER NOT NULL DEFAULT 0,
  started_at INTEGER NOT NULL,
  simulation_duration_ms INTEGER NOT NULL,
  foul_code TEXT,
  final_state_hash TEXT NOT NULL,
  UNIQUE(match_id, shot_number)
);

CREATE TABLE IF NOT EXISTS player_statistics (
  principal TEXT PRIMARY KEY,
  matches_played INTEGER NOT NULL DEFAULT 0,
  wins INTEGER NOT NULL DEFAULT 0,
  losses INTEGER NOT NULL DEFAULT 0,
  balls_pocketed INTEGER NOT NULL DEFAULT 0,
  fouls INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS match_checkpoints (
  match_id TEXT PRIMARY KEY REFERENCES matches(id) ON DELETE CASCADE,
  checkpoint_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
