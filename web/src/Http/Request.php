<?php
declare(strict_types=1);

namespace Pool\Http;

final class Request
{
    public readonly string $method;
    public readonly string $path;
    public readonly array $query;
    public readonly array $headers;
    public readonly array $cookies;
    private ?array $json = null;

    public function __construct()
    {
        $this->method = strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET');
        $this->path = rawurldecode(parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH) ?: '/');
        $this->query = $_GET;
        $this->cookies = $_COOKIE;
        $this->headers = function_exists('getallheaders') ? array_change_key_case(getallheaders(), CASE_LOWER) : [];
    }

    public function json(): array
    {
        if ($this->json !== null) {
            return $this->json;
        }
        $raw = file_get_contents('php://input');
        if ($raw === false || trim($raw) === '') {
            return $this->json = [];
        }
        $decoded = json_decode($raw, true);
        return $this->json = is_array($decoded) ? $decoded : [];
    }

    public function header(string $name): ?string
    {
        return $this->headers[strtolower($name)] ?? null;
    }
}
