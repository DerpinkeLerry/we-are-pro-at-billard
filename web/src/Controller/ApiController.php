<?php
declare(strict_types=1);

namespace Pool\Controller;

use PDO;
use Pool\Http\Request;
use Pool\Http\Response;
use Pool\Lobby\LobbyRepository;
use Pool\Security\Jwt;
use Pool\Service\GameDirectoryClient;
use Pool\Service\RuntimeUrls;
use Pool\Session\Principal;
use Pool\Session\SessionManager;

final class ApiController
{
    private const CUES = ['classic-maple','dark-walnut','carbon-black','tournament-blue','crimson','neon','minimal-white'];

    public function __construct(
        private readonly PDO $pdo,
        private readonly SessionManager $sessions,
        private readonly LobbyRepository $lobbies,
        private readonly GameDirectoryClient $game,
    ) {}

    public function dispatch(Request $request): void
    {
        try {
            $path = $request->path;
            if ($path === '/health' && $request->method === 'GET') {
                $this->pdo->query('SELECT 1')->fetchColumn();
                $gameOk = $this->game->healthy();
                Response::json(['status' => $gameOk ? 'ok' : 'degraded', 'service' => 'pool-arena', 'game' => $gameOk ? 'ok' : 'down'], $gameOk ? 200 : 503);
            }

            if (str_starts_with($path, '/internal/persistence/')) {
                $this->requireInternalSecret($request);
                if ($path === '/internal/persistence/ping' && $request->method === 'GET') {
                    $this->pdo->query('SELECT 1')->fetchColumn();
                    Response::json(['status' => 'ok']);
                }
                if ($request->method !== 'POST') {
                    Response::json(['error' => 'method_not_allowed'], 405);
                }
                if ($path === '/internal/persistence/begin-match') {
                    $this->persistBeginMatch($request);
                }
                if ($path === '/internal/persistence/record-shot') {
                    $this->persistRecordShot($request);
                }
                if ($path === '/internal/persistence/finish-match') {
                    $this->persistFinishMatch($request);
                }
                if ($path === '/internal/persistence/checkpoint') {
                    $this->persistCheckpoint($request);
                }
                if ($path === '/internal/persistence/close-lobby') {
                    $this->persistCloseLobby($request);
                }
                Response::json(['error' => 'not_found'], 404);
            }

            if ($path === '/api/session' && $request->method === 'GET') {
                $this->session();
            }
            if ($path === '/api/guest' && $request->method === 'POST') {
                $this->guest($request);
            }
            if ($path === '/api/register' && $request->method === 'POST') {
                $this->register($request);
            }
            if ($path === '/api/login' && $request->method === 'POST') {
                $this->login($request);
            }
            if ($path === '/api/logout' && $request->method === 'POST') {
                $this->logout($request);
            }
            if ($path === '/api/lobbies' && $request->method === 'GET') {
                $this->listLobbies();
            }
            if ($path === '/api/lobbies' && $request->method === 'POST') {
                $this->createLobby($request);
            }
            if (preg_match('#^/api/lobbies/([A-Z0-9]+)/ticket$#i', $path, $m) && $request->method === 'POST') {
                $this->ticket($request, $m[1]);
            }
            if ($path === '/api/profile' && $request->method === 'GET') {
                $this->profile();
            }
            if ($path === '/api/matches' && $request->method === 'GET') {
                $this->matches();
            }
            Response::json(['error' => 'not_found'], 404);
        } catch (\PDOException $e) {
            $code = (string)$e->getCode();
            if (str_starts_with($code, '23') || str_contains(strtolower($e->getMessage()), 'unique constraint')) {
                Response::json(['error' => 'already_exists'], 409);
            }
            error_log(json_encode(['level'=>'error','service'=>'web','event'=>'db_error','message'=>$e->getMessage()]));
            Response::json(['error' => 'database_error'], 500);
        } catch (\RuntimeException $e) {
            if ($e->getMessage() === 'csrf') {
                Response::json(['error' => 'csrf'], 403);
            }
            error_log(json_encode(['level'=>'error','service'=>'web','event'=>'runtime_error','message'=>$e->getMessage()]));
            Response::json(['error' => 'server_error'], 500);
        }
    }

    private function session(): void
    {
        $p = $this->sessions->ensureGuest();
        Response::json(['principal' => $this->principalPayload($p)]);
    }

    private function guest(Request $request): void
    {
        $p = $this->sessions->ensureGuest();
        $this->sessions->requireCsrf($p, $request->header('x-csrf-token'));
        $nickname = $this->nickname((string)($request->json()['nickname'] ?? ''));
        $p = $this->sessions->updateGuestNickname($p, $nickname);
        Response::json(['principal' => $this->principalPayload($p)]);
    }

    private function register(Request $request): void
    {
        $current = $this->sessions->ensureGuest();
        $this->sessions->requireCsrf($current, $request->header('x-csrf-token'));
        $body = $request->json();
        $username = trim((string)($body['username'] ?? ''));
        $display = $this->nickname((string)($body['displayName'] ?? ''));
        $password = (string)($body['password'] ?? '');
        if (!preg_match('/^[A-Za-z0-9_]{3,32}$/', $username) || strlen($password) < 10 || strlen($password) > 200) {
            Response::json(['error' => 'invalid_registration'], 422);
        }
        $p = $this->sessions->register($username, $display, $password);
        Response::json(['principal' => $this->principalPayload($p)], 201);
    }

    private function login(Request $request): void
    {
        $current = $this->sessions->ensureGuest();
        $this->sessions->requireCsrf($current, $request->header('x-csrf-token'));
        $body = $request->json();
        $p = $this->sessions->login((string)($body['username'] ?? ''), (string)($body['password'] ?? ''));
        if (!$p) {
            Response::json(['error' => 'invalid_credentials'], 401);
        }
        Response::json(['principal' => $this->principalPayload($p)]);
    }

    private function logout(Request $request): void
    {
        $p = $this->requirePrincipal();
        $this->sessions->requireCsrf($p, $request->header('x-csrf-token'));
        $this->sessions->logout();
        $new = $this->sessions->ensureGuest();
        Response::json(['principal' => $this->principalPayload($new)]);
    }

    private function listLobbies(): void
    {
        $rows = $this->lobbies->listOpen();
        $runtime = [];
        foreach ($this->game->runtimeLobbies() as $row) {
            if (isset($row['code'])) {
                $runtime[strtoupper((string)$row['code'])] = $row;
            }
        }
        foreach ($rows as &$row) {
            $r = $runtime[strtoupper((string)$row['short_code'])] ?? null;
            $row['runtime'] = $r ?? ['state'=>'WAITING','players'=>0,'spectators'=>0,'queueSize'=>0];
            $row['created_at'] = $this->isoTime((int)$row['created_at']);
            unset($row['password_hash']);
        }
        Response::json(['lobbies' => $rows]);
    }

    private function createLobby(Request $request): void
    {
        $p = $this->requirePrincipal();
        $this->sessions->requireCsrf($p, $request->header('x-csrf-token'));
        $body = $request->json();
        $name = trim((string)($body['name'] ?? ''));
        if (mb_strlen($name) < 3 || mb_strlen($name) > 48) {
            Response::json(['error' => 'invalid_name'], 422);
        }
        $visibility = ($body['visibility'] ?? 'public') === 'private' ? 'private' : 'public';
        $password = trim((string)($body['password'] ?? ''));
        if ($visibility === 'private' && (strlen($password) < 4 || strlen($password) > 128)) {
            Response::json(['error' => 'invalid_password'], 422);
        }
        $timer = (int)($body['shotTimerSeconds'] ?? 45);
        if (!in_array($timer, [0,30,45,60], true)) {
            Response::json(['error' => 'invalid_timer'], 422);
        }
        $row = $this->lobbies->create($p->key(), $name, $visibility, $visibility === 'private' ? $password : null, $timer);
        $row['created_at'] = $this->isoTime((int)$row['created_at']);
        error_log(json_encode(['level'=>'info','service'=>'web','event'=>'lobby_created','code'=>$row['short_code']]));
        Response::json(['lobby' => $row, 'url' => '/lobby/' . $row['short_code']], 201);
    }

    private function ticket(Request $request, string $code): void
    {
        $p = $this->requirePrincipal();
        $this->sessions->requireCsrf($p, $request->header('x-csrf-token'));
        $lobby = $this->lobbies->findByCode($code);
        if (!$lobby) {
            Response::json(['error' => 'lobby_not_found'], 404);
        }
        $body = $request->json();
        if ($lobby['visibility'] === 'private' && !password_verify((string)($body['password'] ?? ''), (string)$lobby['password_hash'])) {
            Response::json(['error' => 'invalid_lobby_password'], 403);
        }
        $nickname = $p->type === 'user' ? $p->nickname : $this->nickname((string)($body['nickname'] ?? $p->nickname));
        $cue = (string)($body['cueSkin'] ?? 'classic-maple');
        if (!in_array($cue, self::CUES, true)) {
            Response::json(['error' => 'invalid_cue'], 422);
        }
        $now = time();
        $payload = [
            'iss' => 'pool-web', 'aud' => 'pool-game', 'sub' => $p->key(), 'iat' => $now, 'nbf' => $now - 2,
            'exp' => $now + 60, 'jti' => bin2hex(random_bytes(16)),
            'principalType' => $p->type, 'principalId' => $p->id, 'nickname' => $nickname,
            'lobbyId' => $lobby['id'], 'lobbyCode' => $lobby['short_code'], 'lobbyName' => $lobby['name'],
            'cueSkin' => $cue, 'shotTimerSeconds' => (int)$lobby['shot_timer_seconds'],
            'rulesetVersion' => $lobby['ruleset_version'], 'tableConfigVersion' => $lobby['table_config_version'],
        ];
        $token = Jwt::encode($payload, getenv('JOIN_TOKEN_SECRET') ?: '');
        Response::json(['token' => $token, 'wsUrl' => RuntimeUrls::gameWebSocket()]);
    }

    private function profile(): void
    {
        $p = $this->requirePrincipal();
        $stmt = $this->pdo->prepare('SELECT matches_played,wins,losses,balls_pocketed,fouls FROM player_statistics WHERE principal=:p');
        $stmt->execute(['p' => $p->key()]);
        $stats = $stmt->fetch() ?: ['matches_played'=>0,'wins'=>0,'losses'=>0,'balls_pocketed'=>0,'fouls'=>0];
        Response::json(['principal' => $this->principalPayload($p), 'statistics' => $stats]);
    }

    private function matches(): void
    {
        $p = $this->requirePrincipal();
        $stmt = $this->pdo->prepare(
            'SELECT m.id,m.started_at,m.finished_at,m.winner_principal,m.end_reason,m.duration_ms,l.name AS lobby_name,
                    mp.nickname,mp.result,mp.fouls,mp.balls_pocketed
             FROM match_players mp JOIN matches m ON m.id=mp.match_id JOIN lobbies l ON l.id=m.lobby_id
             WHERE mp.principal=:p ORDER BY m.started_at DESC LIMIT 50'
        );
        $stmt->execute(['p' => $p->key()]);
        $rows = $stmt->fetchAll();
        foreach ($rows as &$row) {
            $row['started_at'] = $this->isoTime((int)$row['started_at']);
            $row['finished_at'] = $row['finished_at'] !== null ? $this->isoTime((int)$row['finished_at']) : null;
        }
        Response::json(['matches' => $rows]);
    }

    private function persistBeginMatch(Request $request): void
    {
        $b = $request->json();
        $players = is_array($b['players'] ?? null) ? $b['players'] : [];
        if (count($players) !== 2) {
            Response::json(['error'=>'invalid_payload'], 422);
        }
        $this->pdo->beginTransaction();
        try {
            $stmt = $this->pdo->prepare(
                'INSERT OR IGNORE INTO matches(id,lobby_id,ruleset_version,physics_version,table_config_version,engine_version,rack_seed,started_at)
                 VALUES(:id,:lobby,:rules,:physics,:table,:engine,:seed,:started)'
            );
            $stmt->execute([
                'id'=>(string)$b['matchId'], 'lobby'=>(string)$b['lobbyId'], 'rules'=>(string)$b['rulesetVersion'],
                'physics'=>(string)$b['physicsVersion'], 'table'=>(string)$b['tableConfigVersion'], 'engine'=>(string)$b['engineVersion'],
                'seed'=>(int)$b['rackSeed'], 'started'=>$this->timestamp($b['startedAt'] ?? null),
            ]);
            $ins = $this->pdo->prepare(
                'INSERT OR IGNORE INTO match_players(match_id,seat,principal,nickname,cue_skin) VALUES(:match,:seat,:principal,:nickname,:cue)'
            );
            foreach (array_values($players) as $seat => $player) {
                $ins->execute([
                    'match'=>(string)$b['matchId'], 'seat'=>$seat, 'principal'=>(string)($player['principal'] ?? ''),
                    'nickname'=>(string)($player['nickname'] ?? ''), 'cue'=>(string)($player['cueSkin'] ?? 'classic-maple'),
                ]);
            }
            $this->pdo->commit();
        } catch (\Throwable $e) {
            $this->pdo->rollBack();
            throw $e;
        }
        Response::json(['status'=>'ok']);
    }

    private function persistRecordShot(Request $request): void
    {
        $b = $request->json();
        $stmt = $this->pdo->prepare(
            'INSERT OR IGNORE INTO shots(match_id,shot_number,principal,aim_angle,power,cue_offset_x,cue_offset_y,called_ball,called_pocket,safety,started_at,simulation_duration_ms,foul_code,final_state_hash)
             VALUES(:match,:shot,:principal,:aim,:power,:ox,:oy,:ball,:pocket,:safety,:started,:duration,:foul,:hash)'
        );
        $stmt->execute([
            'match'=>(string)$b['matchId'], 'shot'=>(int)$b['shotNumber'], 'principal'=>(string)$b['principal'],
            'aim'=>(float)$b['aimAngle'], 'power'=>(float)$b['power'], 'ox'=>(float)$b['cueOffsetX'], 'oy'=>(float)$b['cueOffsetY'],
            'ball'=>isset($b['calledBall']) && (int)$b['calledBall'] > 0 ? (int)$b['calledBall'] : null,
            'pocket'=>isset($b['calledPocket']) && (int)$b['calledPocket'] >= 0 ? (int)$b['calledPocket'] : null,
            'safety'=>!empty($b['safety']) ? 1 : 0, 'started'=>$this->timestamp($b['startedAt'] ?? null),
            'duration'=>(int)$b['simulationDurationMs'], 'foul'=>($b['foulCode'] ?? '') !== '' ? (string)$b['foulCode'] : null,
            'hash'=>(string)$b['finalStateHash'],
        ]);
        Response::json(['status'=>'ok']);
    }

    private function persistFinishMatch(Request $request): void
    {
        $b = $request->json();
        $players = is_array($b['players'] ?? null) ? $b['players'] : [];
        $this->pdo->beginTransaction();
        try {
            $stmt = $this->pdo->prepare(
                'UPDATE matches SET finished_at=:finished,winner_principal=:winner,loser_principal=:loser,end_reason=:reason,duration_ms=:duration WHERE id=:id'
            );
            $stmt->execute([
                'finished'=>$this->timestamp($b['finishedAt'] ?? null), 'winner'=>(string)($b['winnerPrincipal'] ?? ''),
                'loser'=>(string)($b['loserPrincipal'] ?? ''), 'reason'=>(string)($b['endReason'] ?? ''),
                'duration'=>(int)($b['durationMs'] ?? 0), 'id'=>(string)$b['matchId'],
            ]);
            $playerUpdate = $this->pdo->prepare(
                'UPDATE match_players SET assigned_group=:grp,fouls=:fouls,balls_pocketed=:balls,result=:result WHERE match_id=:match AND seat=:seat'
            );
            $stats = $this->pdo->prepare(
                'INSERT INTO player_statistics(principal,matches_played,wins,losses,balls_pocketed,fouls,updated_at)
                 VALUES(:principal,1,:wins,:losses,:balls,:fouls,:updated)
                 ON CONFLICT(principal) DO UPDATE SET
                   matches_played=player_statistics.matches_played+1,
                   wins=player_statistics.wins+excluded.wins,
                   losses=player_statistics.losses+excluded.losses,
                   balls_pocketed=player_statistics.balls_pocketed+excluded.balls_pocketed,
                   fouls=player_statistics.fouls+excluded.fouls,
                   updated_at=excluded.updated_at'
            );
            foreach ($players as $player) {
                $result = (string)($player['result'] ?? 'loss');
                $playerUpdate->execute([
                    'grp'=>(string)($player['group'] ?? 'open'), 'fouls'=>(int)($player['fouls'] ?? 0), 'balls'=>(int)($player['ballsPocketed'] ?? 0),
                    'result'=>$result, 'match'=>(string)$b['matchId'], 'seat'=>(int)($player['seat'] ?? 0),
                ]);
                $stats->execute([
                    'principal'=>(string)($player['principal'] ?? ''), 'wins'=>$result === 'win' ? 1 : 0, 'losses'=>$result === 'win' ? 0 : 1,
                    'balls'=>(int)($player['ballsPocketed'] ?? 0), 'fouls'=>(int)($player['fouls'] ?? 0), 'updated'=>time(),
                ]);
            }
            $del = $this->pdo->prepare('DELETE FROM match_checkpoints WHERE match_id=:match');
            $del->execute(['match'=>(string)$b['matchId']]);
            $this->pdo->commit();
        } catch (\Throwable $e) {
            $this->pdo->rollBack();
            throw $e;
        }
        Response::json(['status'=>'ok']);
    }

    private function persistCheckpoint(Request $request): void
    {
        $b = $request->json();
        $json = json_encode($b['state'] ?? null, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR);
        $stmt = $this->pdo->prepare(
            'INSERT INTO match_checkpoints(match_id,checkpoint_json,updated_at) VALUES(:match,:json,:updated)
             ON CONFLICT(match_id) DO UPDATE SET checkpoint_json=excluded.checkpoint_json,updated_at=excluded.updated_at'
        );
        $stmt->execute(['match'=>(string)$b['matchId'], 'json'=>$json, 'updated'=>time()]);
        Response::json(['status'=>'ok']);
    }

    private function persistCloseLobby(Request $request): void
    {
        $b = $request->json();
        $stmt = $this->pdo->prepare('UPDATE lobbies SET closed_at=COALESCE(closed_at,:closed) WHERE id=:id');
        $stmt->execute(['closed'=>time(), 'id'=>(string)$b['lobbyId']]);
        Response::json(['status'=>'ok']);
    }

    private function requireInternalSecret(Request $request): void
    {
        $expected = getenv('GAME_INTERNAL_SECRET') ?: '';
        $provided = $request->header('x-internal-secret') ?? '';
        if ($expected === '' || !hash_equals($expected, $provided)) {
            Response::json(['error'=>'forbidden'], 403);
        }
    }

    private function requirePrincipal(): Principal
    {
        return $this->sessions->current() ?? $this->sessions->ensureGuest();
    }

    private function nickname(string $value): string
    {
        $value = trim(preg_replace('/\s+/u', ' ', $value) ?? '');
        if (!preg_match('/^[\p{L}\p{N}_ .-]{2,24}$/u', $value)) {
            Response::json(['error' => 'invalid_nickname'], 422);
        }
        return $value;
    }

    private function principalPayload(Principal $p): array
    {
        return ['type'=>$p->type,'id'=>$p->id,'nickname'=>$p->nickname,'username'=>$p->username,'csrfToken'=>$p->csrfToken];
    }

    private function isoTime(int $unix): string
    {
        return gmdate('Y-m-d\TH:i:s\Z', $unix);
    }

    private function timestamp(mixed $value): int
    {
        if (is_int($value) || is_float($value) || (is_string($value) && ctype_digit($value))) {
            return (int)$value;
        }
        if (is_string($value) && $value !== '') {
            $ts = strtotime($value);
            if ($ts !== false) {
                return $ts;
            }
        }
        return time();
    }
}
