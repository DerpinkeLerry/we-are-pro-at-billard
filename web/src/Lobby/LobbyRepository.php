<?php
declare(strict_types=1);

namespace Pool\Lobby;

use PDO;
use Pool\Support\Uuid;

final class LobbyRepository
{
    public function __construct(private readonly PDO $pdo) {}

    public function listOpen(): array
    {
        $stmt = $this->pdo->query(
            "SELECT id, short_code, name, visibility, shot_timer_seconds, ruleset_version, table_config_version, created_at
             FROM lobbies WHERE closed_at IS NULL ORDER BY created_at DESC LIMIT 100"
        );
        return $stmt->fetchAll();
    }

    public function findByCode(string $code): ?array
    {
        $stmt = $this->pdo->prepare(
            "SELECT id, short_code, creator_principal, name, visibility, password_hash, shot_timer_seconds,
                    ruleset_version, table_config_version, created_at
             FROM lobbies WHERE short_code=:code AND closed_at IS NULL"
        );
        $stmt->execute(['code' => strtoupper($code)]);
        return $stmt->fetch() ?: null;
    }

    public function create(string $principal, string $name, string $visibility, ?string $password, int $shotTimer): array
    {
        for ($attempt = 0; $attempt < 8; $attempt++) {
            $id = Uuid::v4();
            $code = $this->shortCode();
            $hash = $visibility === 'private' && $password !== null ? password_hash($password, PASSWORD_DEFAULT) : null;
            $stmt = $this->pdo->prepare(
                "INSERT OR IGNORE INTO lobbies(id,short_code,creator_principal,name,visibility,password_hash,shot_timer_seconds,ruleset_version,table_config_version,created_at)
                VALUES(:id,:code,:principal,:name,:visibility,:hash,:timer,'wpa-8ball-v1','pool-7ft-v2',:created)"
            );
            $stmt->execute([
                'id' => $id,
                'code' => $code,
                'principal' => $principal,
                'name' => $name,
                'visibility' => $visibility,
                'hash' => $hash,
                'timer' => $shotTimer,
                'created' => time(),
            ]);
            if ($stmt->rowCount() === 1) {
                $get = $this->pdo->prepare('SELECT id,short_code,name,visibility,shot_timer_seconds,ruleset_version,table_config_version,created_at FROM lobbies WHERE id=:id');
                $get->execute(['id' => $id]);
                return $get->fetch();
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
