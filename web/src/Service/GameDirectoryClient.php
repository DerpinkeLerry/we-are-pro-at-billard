<?php
declare(strict_types=1);

namespace Pool\Service;

final class GameDirectoryClient
{
    public function runtimeLobbies(): array
    {
        $body = $this->request('/internal/lobbies', true);
        $decoded = $body !== null ? json_decode($body, true) : null;
        return is_array($decoded) && isset($decoded['lobbies']) && is_array($decoded['lobbies']) ? $decoded['lobbies'] : [];
    }

    public function healthy(): bool
    {
        $body = $this->request('/ping', false, 0.75);
        if ($body === null) {
            return false;
        }
        $decoded = json_decode($body, true);
        return is_array($decoded) && ($decoded['status'] ?? null) === 'ok';
    }

    private function request(string $path, bool $withSecret, float $timeout = 1.5): ?string
    {
        $base = rtrim(getenv('GAME_INTERNAL_URL') ?: 'http://127.0.0.1:8081', '/');
        $headers = "Accept: application/json\r\n";
        if ($withSecret) {
            $headers .= 'X-Internal-Secret: ' . (getenv('GAME_INTERNAL_SECRET') ?: '') . "\r\n";
        }
        $ctx = stream_context_create(['http' => [
            'method' => 'GET',
            'timeout' => $timeout,
            'ignore_errors' => true,
            'header' => $headers,
        ]]);
        $body = @file_get_contents($base . $path, false, $ctx);
        return $body === false ? null : $body;
    }
}
