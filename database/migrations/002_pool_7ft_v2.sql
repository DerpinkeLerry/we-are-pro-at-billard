UPDATE lobbies
SET table_config_version = 'pool-7ft-v2'
WHERE closed_at IS NULL AND table_config_version = 'wpa-9ft-v1';
