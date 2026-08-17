<?php
declare(strict_types=1);

namespace Pool\Service;

final class GameDirectoryClient
{
    public function runtimeLobbies(): array
    {
        $base = rtrim(getenv('GAME_INTERNAL_URL') ?: 'http://game:8081', '/');
        $secret = getenv('GAME_INTERNAL_SECRET') ?: '';
        $ctx = stream_context_create(['http' => [
            'method' => 'GET',
            'timeout' => 1.5,
            'ignore_errors' => true,
            'header' => "X-Internal-Secret: {$secret}\r\nAccept: application/json\r\n",
        ]]);
        $body = @file_get_contents($base . '/internal/lobbies', false, $ctx);
        if ($body === false) {
            return [];
        }
        $decoded = json_decode($body, true);
        return is_array($decoded) && isset($decoded['lobbies']) && is_array($decoded['lobbies']) ? $decoded['lobbies'] : [];
    }
}
