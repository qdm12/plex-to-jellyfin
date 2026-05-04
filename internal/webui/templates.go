package webui

import (
	"html/template"
	"strings"
)

type migrateSettingsData struct {
	ErrorMessage   string
	PlexURL        string
	PlexToken      string
	PlexLibraries  string
	JellyfinURL    string
	JellyfinAPIKey string
	JellyfinUser   string
	DryRun         bool
}

type getPlexTokenData struct {
	ErrorMessage string
	PlexUsername string
	PlexOTP      string
}

func migrateSettingsDataFromSettings(settings MigrationSettings) migrateSettingsData {
	return migrateSettingsData{
		PlexURL:        settings.PlexURL,
		PlexToken:      settings.PlexToken,
		PlexLibraries:  strings.Join(settings.PlexLibraries, ","),
		JellyfinURL:    settings.JellyfinURL,
		JellyfinAPIKey: settings.JellyfinAPIKey,
		JellyfinUser:   settings.JellyfinUser,
		DryRun:         settings.DryRun,
	}
}

//nolint:gochecknoglobals
var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plex to Jellyfin</title>
  <script src="https://unpkg.com/htmx.org@1.9.12"></script>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f4ef;
      --surface: #fffdf7;
      --surface-2: #f0ebe0;
      --text: #1f1f1b;
      --muted: #5d5d53;
      --accent: #1f6f5e;
      --accent-strong: #145045;
      --danger: #992b2b;
      --border: #d6cfbf;
    }
    body {
      margin: 0;
      font-family: "IBM Plex Sans", "Segoe UI", sans-serif;
      color: var(--text);
      background: radial-gradient(circle at top right, #ebe4d4, var(--bg) 60%);
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
    }
    main {
      width: min(860px, 100%);
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 16px;
      box-shadow: 0 12px 36px rgba(31, 31, 27, 0.08);
      padding: 28px;
    }
    h1 {
      margin: 0;
      font-size: clamp(1.6rem, 4vw, 2.5rem);
      letter-spacing: -0.02em;
      font-family: "IBM Plex Serif", "Georgia", serif;
    }
    p {
      color: var(--muted);
      line-height: 1.5;
    }
    .step {
      margin-top: 20px;
      padding: 20px;
      border-radius: 12px;
      border: 1px solid var(--border);
      background: linear-gradient(170deg, var(--surface), var(--surface-2));
      animation: rise .28s ease-out;
    }
    @keyframes rise {
      from {opacity: 0; transform: translateY(10px);}
      to {opacity: 1; transform: translateY(0);}
    }
    .row {
      display: grid;
      gap: 12px;
      margin: 14px 0;
    }
    label {
      font-weight: 600;
      font-size: .95rem;
    }
    input {
      width: 100%;
      box-sizing: border-box;
      border-radius: 8px;
      border: 1px solid var(--border);
      background: #fff;
      padding: 10px 12px;
      font-size: .95rem;
      font-family: "IBM Plex Mono", monospace;
    }
    .button {
      margin-top: 10px;
      border: 0;
      border-radius: 10px;
      background: var(--accent);
      color: #fff;
      font-weight: 700;
      padding: 10px 16px;
      cursor: pointer;
      transition: background .15s ease-in-out;
    }
    .button:hover {
      background: var(--accent-strong);
    }
    pre {
      margin: 0;
      border-radius: 10px;
      border: 1px solid var(--border);
      background: #161b1a;
      color: #d6f5e2;
      padding: 12px;
      height: 320px;
      overflow-y: auto;
      font-family: "IBM Plex Mono", monospace;
      font-size: .85rem;
      line-height: 1.35;
    }
    .error {
      color: var(--danger);
      font-weight: 600;
      margin-bottom: 8px;
    }
    .inline-check {
      display: flex;
      align-items: center;
      gap: 12px;
      font-size: .95rem;
      color: var(--text);
      margin: 16px 0 20px 0;
      padding: 14px;
      border: 1px solid var(--border);
      border-radius: 8px;
      background: rgba(31, 111, 94, 0.04);
    }
    .inline-check input[type="checkbox"] {
      width: 20px;
      height: 20px;
      cursor: pointer;
      accent-color: var(--accent);
      flex-shrink: 0;
    }
    .inline-check label {
      cursor: pointer;
      user-select: none;
      font-size: 0.95rem;
      margin: 0;
    }
    @media (max-width: 640px) {
      body { padding: 12px; }
      main { padding: 18px; }
    }
  </style>
</head>
<body>
  <main>
    <h1>Plex to Jellyfin</h1>
    <section id="wizard">{{.SelectStepHTML}}</section>
  </main>
</body>
</html>`))

//nolint:gochecknoglobals
var selectStepHTML = template.HTML(`<div class="step">
  <h2>Step 1: Choose an action</h2>
  <p>Choose what you want to do.</p>
  <button class="button"
          hx-get="/step/get-plex-token/settings"
          hx-target="#wizard"
          hx-swap="innerHTML">
    Obtain Plex token
  </button>
  <button class="button"
          hx-get="/step/migrate/settings"
          hx-target="#wizard"
          hx-swap="innerHTML">
    migrate
  </button>
</div>`)

//nolint:gochecknoglobals
var getPlexTokenTemplate = template.Must(template.New("get-plex-token").Parse(`<div class="step">
  <h2>Step 2: Get Plex token</h2>
  <p>Sign in with the Plex user credentials to retrieve the Plex token.</p>
  {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
  <form hx-post="/step/get-plex-token/fetch" hx-target="#wizard" hx-swap="innerHTML">
    <div class="row">
      <label for="plex-username">Plex username or email</label>
      <input id="plex-username" name="plex_username" value="{{.PlexUsername}}" required>
    </div>
    <div class="row">
      <label for="plex-password">Plex password</label>
      <input id="plex-password" name="plex_password" type="password" autocomplete="current-password" required>
    </div>
    <div class="row">
      <label for="plex-otp">Plex 2FA code (optional)</label>
            <input id="plex-otp" name="plex_otp" value="{{.PlexOTP}}"
              inputmode="numeric" pattern="[0-9]*" autocomplete="one-time-code">
    </div>
    <button class="button" type="submit">Fetch token</button>
  </form>
</div>`))

//nolint:gochecknoglobals
var getPlexTokenResultTemplate = template.Must(template.New("get-plex-token-result").Parse(`<div class="step">
  <h2>Step 3: Plex token</h2>
  {{if .SendTo}}<p>Send this token to {{.SendTo}}</p>{{else}}<p>Share this token with your admin so
  they can run your migration.</p>{{end}}
  <div class="row">
    <label for="plex-token-result">Plex token</label>
    <input id="plex-token-result" value="{{.PlexToken}}" readonly>
  </div>
  <button class="button" type="button"
          onclick="navigator.clipboard.writeText(document.getElementById('plex-token-result').value)">
    Copy token
  </button>
</div>`))

//nolint:gochecknoglobals
var migrateSettingsTemplate = template.Must(template.New("migrate-settings").Parse(`<div class="step">
  <h2>Step 2: Migrate settings</h2>
  <p>Review and edit the values, then proceed to start migration.</p>
  {{if .ErrorMessage}}<div class="error">{{.ErrorMessage}}</div>{{end}}
  <form hx-post="/step/migrate/start" hx-target="#wizard" hx-swap="innerHTML">
    <div class="row">
      <label for="plex-url">Plex URL</label>
      <input id="plex-url" name="plex_url" value="{{.PlexURL}}" placeholder="http://plex:32400" required>
    </div>
    <div class="row">
      <label for="plex-token">Plex token</label>
      <input id="plex-token" name="plex_token" value="{{.PlexToken}}" required>
    </div>
    <div class="row">
      <label for="plex-libraries">Plex libraries (comma-separated)</label>
      <input id="plex-libraries" name="plex_libraries" value="{{.PlexLibraries}}" placeholder="Movies,TV Shows">
    </div>
    <div class="row">
      <label for="jellyfin-url">Jellyfin URL</label>
      <input id="jellyfin-url" name="jellyfin_url" value="{{.JellyfinURL}}" placeholder="http://jellyfin:8096" required>
    </div>
    <div class="row">
      <label for="jellyfin-user">Jellyfin user</label>
      <input id="jellyfin-user" name="jellyfin_user" value="{{.JellyfinUser}}" required>
    </div>
    <div class="row">
      <label for="jellyfin-api-key">Jellyfin API key</label>
      <input id="jellyfin-api-key" name="jellyfin_api_key" value="{{.JellyfinAPIKey}}" required>
    </div>
    <div class="inline-check">
      <input type="checkbox" id="dry-run" name="dry_run" {{if .DryRun}}checked{{end}}>
      <label for="dry-run">Dry run (safe preview without writing to Jellyfin)</label>
    </div>
    <button class="button" type="submit">Proceed</button>
  </form>
</div>`))

//nolint:gochecknoglobals
var migrateProgressTemplate = template.Must(template.New("migrate-progress").Parse(`<div class="step">
  <h2>Step 3: Migration in progress</h2>
  <p id="migration-status">Connecting to log stream...</p>
  <pre id="migration-log"></pre>
  <script>
    (function() {
      var status = document.getElementById('migration-status');
      var log = document.getElementById('migration-log');
      var stream = new EventSource('/step/migrate/logs?id={{.JobID}}');

      stream.addEventListener('log', function(event) {
        log.appendChild(document.createTextNode(event.data + '\n'));
        log.scrollTop = log.scrollHeight;
        status.textContent = 'Migration is running...';
      });

      stream.addEventListener('done', function(event) {
        status.textContent = event.data;
        stream.close();
      });

      stream.onerror = function() {
        status.textContent = 'Log stream disconnected';
        stream.close();
      };
    })();
  </script>
</div>`))
