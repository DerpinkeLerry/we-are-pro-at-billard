<?php
declare(strict_types=1);

namespace Pool\Controller;

use PDO;
use Pool\Http\Request;
use Pool\Http\Response;
use Pool\Lobby\LobbyRepository;
use Pool\Security\Jwt;
use Pool\Service\GameDirectoryClient;
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
                Response::json(['status' => 'ok', 'service' => 'web']);
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
            $code = $e->getCode();
            if ($code === '23505') {
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
            $r = $runtime[strtoupper($row['short_code'])] ?? null;
            $row['runtime'] = $r ?? ['state'=>'WAITING','players'=>0,'spectators'=>0,'queueSize'=>0];
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
        Response::json(['token' => $token, 'wsUrl' => getenv('GAME_PUBLIC_WS_URL') ?: 'ws://localhost:8081/ws']);
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
            "SELECT m.id::text,m.started_at,m.finished_at,m.winner_principal,m.end_reason,m.duration_ms,l.name AS lobby_name,
                    mp.nickname,mp.result,mp.fouls,mp.balls_pocketed
             FROM match_players mp JOIN matches m ON m.id=mp.match_id JOIN lobbies l ON l.id=m.lobby_id
             WHERE mp.principal=:p ORDER BY m.started_at DESC LIMIT 50"
        );
        $stmt->execute(['p' => $p->key()]);
        Response::json(['matches' => $stmt->fetchAll()]);
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
}
