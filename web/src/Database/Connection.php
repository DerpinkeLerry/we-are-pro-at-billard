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

        $path = getenv('SQLITE_PATH') ?: '/var/lib/pool/pool.sqlite';
        $directory = dirname($path);
        if (!is_dir($directory) && !@mkdir($directory, 0770, true) && !is_dir($directory)) {
            throw new RuntimeException('Cannot create SQLite directory');
        }

        self::$pdo = new PDO('sqlite:' . $path, null, null, [
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            PDO::ATTR_EMULATE_PREPARES => false,
        ]);
        self::$pdo->exec('PRAGMA foreign_keys = ON');
        self::$pdo->exec('PRAGMA journal_mode = WAL');
        self::$pdo->exec('PRAGMA synchronous = NORMAL');
        self::$pdo->exec('PRAGMA busy_timeout = 5000');
        return self::$pdo;
    }
}
