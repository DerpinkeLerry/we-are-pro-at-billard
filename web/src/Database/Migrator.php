<?php
declare(strict_types=1);

namespace Pool\Database;

use PDO;
use RuntimeException;

final class Migrator
{
    public static function run(PDO $pdo, string $directory): array
    {
        $pdo->exec('CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)');
        $files = glob(rtrim($directory, '/') . '/*.sql') ?: [];
        sort($files, SORT_STRING);
        $applied = [];
        foreach ($files as $file) {
            $version = basename($file);
            $check = $pdo->prepare('SELECT 1 FROM schema_migrations WHERE version = :version');
            $check->execute(['version' => $version]);
            if ($check->fetchColumn()) {
                continue;
            }
            $sql = file_get_contents($file);
            if ($sql === false) {
                throw new RuntimeException("Cannot read migration {$file}");
            }
            $pdo->beginTransaction();
            try {
                $pdo->exec($sql);
                $stmt = $pdo->prepare('INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(:version, :applied)');
                $stmt->execute(['version' => $version, 'applied' => time()]);
                $pdo->commit();
                $applied[] = $version;
            } catch (\Throwable $e) {
                $pdo->rollBack();
                throw $e;
            }
        }
        return $applied;
    }
}
