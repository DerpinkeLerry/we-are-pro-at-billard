<?php
declare(strict_types=1);

namespace Pool\Lobby;

use PDO;

final class LobbyRepository
{
    public function __construct(private readonly PDO $pdo) {}

    public function listOpen(): array
    {
        $stmt = $this->pdo->query(
            "SELECT id::text, short_code, name, visibility, shot_timer_seconds, ruleset_version, table_config_version, created_at
             FROM lobbies WHERE closed_at IS NULL ORDER BY created_at DESC LIMIT 100"
        );
        return $stmt->fetchAll();
    }

    public function findByCode(string $code): ?array
    {
        $stmt = $this->pdo->prepare(
            "SELECT id::text, short_code, creator_principal, name, visibility, password_hash, shot_timer_seconds,
                    ruleset_version, table_config_version, created_at
             FROM lobbies WHERE short_code=:code AND closed_at IS NULL"
        );
        $stmt->execute(['code' => strtoupper($code)]);
        return $stmt->fetch() ?: null;
    }

    public function create(string $principal, string $name, string $visibility, ?string $password, int $shotTimer): array
    {
        for ($attempt = 0; $attempt < 8; $attempt++) {
            $code = $this->shortCode();
            $hash = $visibility === 'private' && $password !== null ? password_hash($password, PASSWORD_DEFAULT) : null;
            $stmt = $this->pdo->prepare(
                "INSERT INTO lobbies(short_code,creator_principal,name,visibility,password_hash,shot_timer_seconds)
                 VALUES(:code,:principal,:name,:visibility,:hash,:timer)
                 ON CONFLICT(short_code) DO NOTHING
                 RETURNING id::text, short_code, name, visibility, shot_timer_seconds, ruleset_version, table_config_version, created_at"
            );
            $stmt->execute([
                'code' => $code,
                'principal' => $principal,
                'name' => $name,
                'visibility' => $visibility,
                'hash' => $hash,
                'timer' => $shotTimer,
            ]);
            if ($row = $stmt->fetch()) {
                return $row;
            }
        }
        throw new \RuntimeException('Could not allocate lobby code');
    }

    private function shortCode(): string
    {
        $alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
        $raw = random_bytes(7);
        $code = '';
        for ($i = 0; $i < 7; $i++) {
            $code .= $alphabet[ord($raw[$i]) % strlen($alphabet)];
        }
        return $code;
    }
}
