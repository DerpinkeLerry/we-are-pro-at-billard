UPDATE lobbies
SET ruleset_version = 'red-yellow-8ball-v1'
WHERE closed_at IS NULL AND ruleset_version = 'wpa-8ball-v1';
