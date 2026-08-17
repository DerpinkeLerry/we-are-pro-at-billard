<?php
declare(strict_types=1);

namespace Pool\Database;

use PDO;
use RuntimeException;

final class Connection
{
    private static ?PDO $pdo = null;

    public static function get(): PDO
    {
        if (self::$pdo instanceof PDO) {
            return self::$pdo;
        }

        $url = getenv('DATABASE_URL') ?: 'postgres://pool:pool@db:5432/pool?sslmode=disable';
        $parts = parse_url($url);
        if ($parts === false || !isset($parts['host'], $parts['path'])) {
            throw new RuntimeException('Invalid DATABASE_URL');
        }
        parse_str($parts['query'] ?? '', $query);
        $db = ltrim($parts['path'], '/');
        $port = (int)($parts['port'] ?? 5432);
        $sslmode = preg_replace('/[^a-z-]/i', '', (string)($query['sslmode'] ?? 'prefer'));
        $dsn = sprintf('pgsql:host=%s;port=%d;dbname=%s;sslmode=%s', $parts['host'], $port, $db, $sslmode);

        self::$pdo = new PDO($dsn, urldecode($parts['user'] ?? ''), urldecode($parts['pass'] ?? ''), [
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            PDO::ATTR_EMULATE_PREPARES => false,
        ]);
        return self::$pdo;
    }
}
