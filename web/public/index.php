<?php
declare(strict_types=1);

require __DIR__ . '/../src/autoload.php';

$joinSecret = getenv('JOIN_TOKEN_SECRET') ?: '';
$internalSecret = getenv('GAME_INTERNAL_SECRET') ?: '';
if (strlen($joinSecret) < 32 || strlen($internalSecret) < 32) {
    error_log(json_encode(['level'=>'critical','service'=>'web','event'=>'invalid_secret_configuration']));
    http_response_code(500);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode(['error'=>'server_misconfigured']);
    exit;
}

use Pool\Controller\ApiController;
use Pool\Database\Connection;
use Pool\Http\Request;
use Pool\Http\Response;
use Pool\Lobby\LobbyRepository;
use Pool\Service\GameDirectoryClient;
use Pool\Session\SessionManager;

header("X-Content-Type-Options: nosniff");
header("Referrer-Policy: strict-origin-when-cross-origin");
header("Permissions-Policy: camera=(), microphone=(), geolocation=()");
$cspNonce = base64_encode(random_bytes(18));
header("Content-Security-Policy: default-src 'self'; script-src 'self' https://cdn.jsdelivr.net 'nonce-{$cspNonce}'; style-src 'self'; img-src 'self' data:; connect-src 'self' ws: wss:; media-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'");

$request = new Request();
$pdo = Connection::get();
$sessions = new SessionManager($pdo);

if ($request->path === '/health' || str_starts_with($request->path, '/api/')) {
    (new ApiController($pdo, $sessions, new LobbyRepository($pdo), new GameDirectoryClient()))->dispatch($request);
}

$principal = $sessions->ensureGuest();
$bootstrap = [
    'principal' => ['type'=>$principal->type,'id'=>$principal->id,'nickname'=>$principal->nickname,'username'=>$principal->username,'csrfToken'=>$principal->csrfToken],
    'path' => $request->path,
    'gameWsUrl' => getenv('GAME_PUBLIC_WS_URL') ?: 'ws://localhost:8081/ws',
    'appEnv' => getenv('APP_ENV') ?: 'development',
    'threeUrl' => 'https://cdn.jsdelivr.net/npm/three@0.185.1/build/three.module.js',
];
ob_start();
$bootstrapJson = json_encode($bootstrap, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR);
require __DIR__ . '/../templates/app.php';
Response::html((string)ob_get_clean());
