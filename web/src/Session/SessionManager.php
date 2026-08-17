<?php
declare(strict_types=1);

namespace Pool\Session;

use PDO;
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

        $stmt = $this->pdo->prepare(
            'SELECT s.user_id::text AS id, u.display_name AS nickname, u.username, s.csrf_token
             FROM auth_sessions s JOIN users u ON u.id=s.user_id
             WHERE s.token_hash=:hash AND s.expires_at>now()'
        );
        $stmt->execute(['hash' => $hash]);
        if ($row = $stmt->fetch()) {
            $this->touch('auth_sessions', $hash);
            return new Principal('user', $row['id'], $row['nickname'], $row['csrf_token'], $row['username']);
        }

        $stmt = $this->pdo->prepare(
            'SELECT id::text, nickname, csrf_token FROM guest_sessions WHERE token_hash=:hash AND expires_at>now()'
        );
        $stmt->execute(['hash' => $hash]);
        if ($row = $stmt->fetch()) {
            $this->touch('guest_sessions', $hash);
            return new Principal('guest', $row['id'], $row['nickname'], $row['csrf_token']);
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
        $stmt = $this->pdo->prepare(
            "INSERT INTO guest_sessions(token_hash,nickname,csrf_token,expires_at)
             VALUES(:hash,:nickname,:csrf,now()+interval '30 days') RETURNING id::text"
        );
        $stmt->execute(['hash' => hash('sha256', $token), 'nickname' => $nickname, 'csrf' => $csrf]);
        $id = (string)$stmt->fetchColumn();
        $this->setCookie($token, 30 * 86400);
        return new Principal('guest', $id, $nickname, $csrf);
    }

    public function updateGuestNickname(Principal $principal, string $nickname): Principal
    {
        if ($principal->type !== 'guest') {
            return $principal;
        }
        $stmt = $this->pdo->prepare('UPDATE guest_sessions SET nickname=:nickname,last_seen_at=now() WHERE id=:id');
        $stmt->execute(['nickname' => $nickname, 'id' => $principal->id]);
        return new Principal('guest', $principal->id, $nickname, $principal->csrfToken);
    }

    public function register(string $username, string $displayName, string $password): Principal
    {
        $hash = password_hash($password, PASSWORD_DEFAULT);
        $this->pdo->beginTransaction();
        try {
            $stmt = $this->pdo->prepare('INSERT INTO users(username,display_name,password_hash) VALUES(:u,:d,:p) RETURNING id::text');
            $stmt->execute(['u' => $username, 'd' => $displayName, 'p' => $hash]);
            $id = (string)$stmt->fetchColumn();
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
        $stmt = $this->pdo->prepare('SELECT id::text, username, display_name, password_hash FROM users WHERE lower(username)=lower(:u)');
        $stmt->execute(['u' => $username]);
        $row = $stmt->fetch();
        if (!$row || !password_verify($password, $row['password_hash'])) {
            return null;
        }
        return $this->createAuthSession($row['id'], $row['display_name'], $row['username']);
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
        $stmt = $this->pdo->prepare(
            "INSERT INTO auth_sessions(user_id,token_hash,csrf_token,expires_at)
             VALUES(:uid,:hash,:csrf,now()+interval '14 days')"
        );
        $stmt->execute(['uid' => $userId, 'hash' => hash('sha256', $token), 'csrf' => $csrf]);
        $this->setCookie($token, 14 * 86400);
        return new Principal('user', $userId, $displayName, $csrf, $username);
    }

    private function touch(string $table, string $hash): void
    {
        $stmt = $this->pdo->prepare("UPDATE {$table} SET last_seen_at=now() WHERE token_hash=:hash AND last_seen_at < now()-interval '1 minute'");
        $stmt->execute(['hash' => $hash]);
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
