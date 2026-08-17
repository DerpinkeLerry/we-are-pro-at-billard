<?php
declare(strict_types=1);

require __DIR__ . '/../src/autoload.php';

use Pool\Database\Connection;
use Pool\Database\Migrator;

$dir = realpath(__DIR__ . '/../../database/migrations');
if ($dir === false) {
    fwrite(STDERR, "Migration directory not found\n");
    exit(1);
}
$applied = Migrator::run(Connection::get(), $dir);
echo $applied ? "Applied: " . implode(', ', $applied) . "\n" : "No migrations pending\n";
