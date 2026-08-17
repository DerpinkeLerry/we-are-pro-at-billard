<?php
declare(strict_types=1);

namespace Pool\Session;

use PDO;
use Pool\Support\Uuid;
use RuntimeException;

final class SessionManager
{
    private const COOKIE = 'pool_session';

    public function __construct(private readonly PDO $pdo) {}

    public function current(): ?Principal
    {
        $token = $_COOKIE[self::COOKIE] ?? '';
        if (!is_string($token) || strlen($token) < 32) {
            return null;
        }
        $hash = hash('sha256', $token);
        $now = time();

        $stmt = $this->pdo->prepare(
            'SELECT s.user_id AS id, u.display_name AS nickname, u.username, s.csrf_token
             FROM auth_sessions s JOIN users u ON u.id=s.user_id
             WHERE s.token_hash=:hash AND s.expires_at>:now'
        );
        $stmt->execute(['hash' => $hash, 'now' => $now]);
        if ($row = $stmt->fetch()) {
            $this->touch('auth_sessions', $hash, $now);
            return new Principal('user', (string)$row['id'], (string)$row['nickname'], (string)$row['csrf_token'], (string)$row['username']);
        }

        $stmt = $this->pdo->prepare(
            'SELECT id, nickname, csrf_token FROM guest_sessions WHERE token_hash=:hash AND expires_at>:now'
        );
        $stmt->execute(['hash' => $hash, 'now' => $now]);
        if ($row = $stmt->fetch()) {
            $this->touch('guest_sessions', $hash, $now);
            return new Principal('guest', (string)$row['id'], (string)$row['nickname'], (string)$row['csrf_token']);
        }
        return null;
    }

    public function ensureGuest(): Principal
    {
        if ($current = $this->current()) {
            return $current;
        }
        $token = self::randomToken(32);
        $csrf = self::randomToken(32);
        $nickname = 'Guest-' . strtoupper(substr(bin2hex(random_bytes(3)), 0, 6));
        $id = Uuid::v4();
        $now = time();
        $stmt = $this->pdo->prepare(
            'INSERT INTO guest_sessions(id,token_hash,nickname,csrf_token,created_at,last_seen_at,expires_at)
             VALUES(:id,:hash,:nickname,:csrf,:created,:seen,:expires)'
        );
        $stmt->execute([
            'id' => $id,
            'hash' => hash('sha256', $token),
            'nickname' => $nickname,
            'csrf' => $csrf,
            'created' => $now,
            'seen' => $now,
            'expires' => $now + 30 * 86400,
        ]);
        $this->setCookie($token, 30 * 86400);
        return new Principal('guest', $id, $nickname, $csrf);
    }

    public function updateGuestNickname(Principal $principal, string $nickname): Principal
    {
        if ($principal->type !== 'guest') {
            return $principal;
        }
        $stmt = $this->pdo->prepare('UPDATE guest_sessions SET nickname=:nickname,last_seen_at=:now WHERE id=:id');
        $stmt->execute(['nickname' => $nickname, 'now' => time(), 'id' => $principal->id]);
        return new Principal('guest', $principal->id, $nickname, $principal->csrfToken);
    }

    public function register(string $username, string $displayName, string $password): Principal
    {
        $hash = password_hash($password, PASSWORD_DEFAULT);
        $this->pdo->beginTransaction();
        try {
            $id = Uuid::v4();
            $now = time();
            $stmt = $this->pdo->prepare('INSERT INTO users(id,username,display_name,password_hash,created_at,updated_at) VALUES(:id,:u,:d,:p,:created,:updated)');
            $stmt->execute(['id' => $id, 'u' => $username, 'd' => $displayName, 'p' => $hash, 'created' => $now, 'updated' => $now]);
            $principal = $this->createAuthSession($id, $displayName, $username);
            $this->pdo->commit();
            return $principal;
        } catch (\Throwable $e) {
            $this->pdo->rollBack();
            throw $e;
        }
    }

    public function login(string $username, string $password): ?Principal
    {
        $stmt = $this->pdo->prepare('SELECT id, username, display_name, password_hash FROM users WHERE username=:u COLLATE NOCASE');
        $stmt->execute(['u' => $username]);
        $row = $stmt->fetch();
        if (!$row || !password_verify($password, (string)$row['password_hash'])) {
            return null;
        }
        return $this->createAuthSession((string)$row['id'], (string)$row['display_name'], (string)$row['username']);
    }

    public function logout(): void
    {
        $token = $_COOKIE[self::COOKIE] ?? '';
        if (is_string($token) && $token !== '') {
            $hash = hash('sha256', $token);
            foreach (['auth_sessions', 'guest_sessions'] as $table) {
                $stmt = $this->pdo->prepare("DELETE FROM {$table} WHERE token_hash=:hash");
                $stmt->execute(['hash' => $hash]);
            }
        }
        setcookie(self::COOKIE, '', $this->cookieOptions(-3600));
    }

    public function requireCsrf(Principal $principal, ?string $provided): void
    {
        if (!is_string($provided) || !hash_equals($principal->csrfToken, $provided)) {
            throw new RuntimeException('csrf');
        }
    }

    private function createAuthSession(string $userId, string $displayName, string $username): Principal
    {
        $token = self::randomToken(32);
        $csrf = self::randomToken(32);
        $now = time();
        $stmt = $this->pdo->prepare(
            'INSERT INTO auth_sessions(id,user_id,token_hash,csrf_token,created_at,last_seen_at,expires_at)
             VALUES(:id,:uid,:hash,:csrf,:created,:seen,:expires)'
        );
        $stmt->execute([
            'id' => Uuid::v4(),
            'uid' => $userId,
            'hash' => hash('sha256', $token),
            'csrf' => $csrf,
            'created' => $now,
            'seen' => $now,
            'expires' => $now + 14 * 86400,
        ]);
        $this->setCookie($token, 14 * 86400);
        return new Principal('user', $userId, $displayName, $csrf, $username);
    }

    private function touch(string $table, string $hash, int $now): void
    {
        $stmt = $this->pdo->prepare("UPDATE {$table} SET last_seen_at=:now WHERE token_hash=:hash AND last_seen_at<:cutoff");
        $stmt->execute(['now' => $now, 'hash' => $hash, 'cutoff' => $now - 60]);
    }

    private function setCookie(string $token, int $ttl): void
    {
        setcookie(self::COOKIE, $token, $this->cookieOptions($ttl));
    }

    private function cookieOptions(int $ttl): array
    {
        return [
            'expires' => time() + $ttl,
            'path' => '/',
            'secure' => (getenv('APP_ENV') ?: 'development') === 'production',
            'httponly' => true,
            'samesite' => 'Lax',
        ];
    }

    private static function randomToken(int $bytes): string
    {
        return rtrim(strtr(base64_encode(random_bytes($bytes)), '+/', '-_'), '=');
    }
}
