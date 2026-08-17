<?php
declare(strict_types=1);

namespace Pool\Session;

final class Principal
{
    public function __construct(
        public readonly string $type,
        public readonly string $id,
        public readonly string $nickname,
        public readonly string $csrfToken,
        public readonly ?string $username = null,
    ) {}

    public function key(): string
    {
        return $this->type . ':' . $this->id;
    }
}
