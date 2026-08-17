<?php
declare(strict_types=1);

namespace Pool\Service;

final class RuntimeUrls
{
    public static function gameWebSocket(): string
    {
        $configured = trim((string)(getenv('GAME_PUBLIC_WS_URL') ?: ''));
        return $configured !== '' ? $configured : '/ws';
    }
}
