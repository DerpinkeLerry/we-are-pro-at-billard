<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
  <meta name="theme-color" content="#091310">
  <title>Pool Arena</title>
  <link rel="stylesheet" href="/assets/css/app.css">
</head>
<body>
  <div id="app" class="app-shell">
    <header class="topbar">
      <a class="brand" href="/" data-nav><span class="brand-mark">8</span><span>POOL ARENA</span></a>
      <nav>
        <a href="/lobbies" data-nav>Lobbies</a>
        <a href="/profile" data-nav>Profile</a>
        <a href="/history" data-nav>Match History</a>
        <button id="settings-button" class="link-button" type="button">Settings</button>
      </nav>
      <div id="identity-chip" class="identity-chip"></div>
    </header>
    <main id="route-root" class="route-root" aria-live="polite"></main>
  </div>
  <div id="modal-root"></div>
  <div id="toast-root" class="toast-root" aria-live="polite"></div>
  <script nonce="<?= htmlspecialchars($cspNonce, ENT_QUOTES) ?>">window.__POOL_BOOTSTRAP__ = <?= $bootstrapJson ?>;</script>
  <script type="module" src="/assets/js/app/main.js"></script>
</body>
</html>
