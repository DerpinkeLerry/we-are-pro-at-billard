<?php
declare(strict_types=1);

namespace Pool\Security;

use RuntimeException;

final class Jwt
{
    public static function encode(array $payload, string $secret): string
    {
        if (strlen($secret) < 32) {
            throw new RuntimeException('JOIN_TOKEN_SECRET must be at least 32 bytes');
        }
        $header = ['alg' => 'HS256', 'typ' => 'JWT'];
        $a = self::b64(json_encode($header, JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR));
        $b = self::b64(json_encode($payload, JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR));
        $sig = hash_hmac('sha256', $a . '.' . $b, $secret, true);
        return $a . '.' . $b . '.' . self::b64($sig);
    }

    private static function b64(string $data): string
    {
        return rtrim(strtr(base64_encode($data), '+/', '-_'), '=');
    }
}
