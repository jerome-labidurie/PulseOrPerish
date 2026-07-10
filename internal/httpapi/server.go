package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"pulseorperish/internal/monitor"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type Server struct {
	log      zerolog.Logger
	password string
	monitor  *monitor.Service
}

func NewServer(log zerolog.Logger, password string, m *monitor.Service) *Server {
	return &Server{log: log, password: password, monitor: m}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/", s.handleIndex)
	r.Get("/status", s.handleStatus)

	r.Group(func(pr chi.Router) {
		pr.Use(s.authMiddleware)
		pr.Post("/alive", s.handleAlive)
	})

	return r
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractPassword(r)
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.password)) != 1 {
			s.log.Warn().Str("remote", r.RemoteAddr).Msg("authentication failed")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAlive(w http.ResponseWriter, r *http.Request) {
	s.log.Debug().Str("remote", r.RemoteAddr).Msg("proof of life request")

	at, err := s.monitor.RegisterProof("http")
	if err != nil {
		s.log.Error().Err(err).Msg("failed to persist proof")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist proof"})
		return
	}
	status := s.monitor.Snapshot(time.Now().UTC())
	writeJSON(w, http.StatusOK, map[string]any{
		"lastProofAt":          at,
		"nextDeletion":         status.NextDeletion,
		"timeRemaining":        status.TimeRemaining,
		"timeRemainingMinutes": status.TimeRemainingMinutes,
		"dryRun":               status.DryRun,
	})
}

func (s *Server) authenticateRequest(r *http.Request) bool {
	provided := s.extractPassword(r)
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.password)) == 1
}

func (s *Server) extractPassword(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}

	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		var payload struct {
			Password string `json:"password"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&payload); err == nil {
			return strings.TrimSpace(payload.Password)
		}
		return ""
	}

	if err := r.ParseForm(); err == nil {
		return strings.TrimSpace(r.FormValue("password"))
	}
	return ""
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.log.Debug().Str("remote", r.RemoteAddr).Msg("status request")
	writeJSON(w, http.StatusOK, s.monitor.Snapshot(time.Now().UTC()))
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>PulseOrPerish</title>
  <style>
    :root {
      color-scheme: light dark;
      --bg-a: #e2e8f0;
      --bg-b: #bfdbfe;
      --glow-a: rgba(59,130,246,0.28);
      --glow-b: rgba(239,68,68,0.24);
      --panel: rgba(255,255,255,0.92);
      --ink: #0b1320;
      --muted: #475569;
      --field-bg: rgba(255,255,255,0.9);
      --field-border: #cbd5e1;
      --accent: #ef4444;
      --secondary: #0ea5e9;
      --ok: #16a34a;
      --status-bg: #e2e8f0;
      --status-ok-bg: #dcfce7;
      --status-ok-ink: #14532d;
      --status-bad-bg: #fee2e2;
      --status-bad-ink: #7f1d1d;
      --shadow: rgba(15,23,42,0.18);
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg-a: #020617;
        --bg-b: #0f172a;
        --glow-a: rgba(30,64,175,0.24);
        --glow-b: rgba(185,28,28,0.18);
        --panel: rgba(15,23,42,0.78);
        --ink: #cbd5e1;
        --muted: #94a3b8;
        --field-bg: rgba(15,23,42,0.62);
        --field-border: rgba(148,163,184,0.42);
        --accent: #b91c1c;
        --secondary: #0369a1;
        --status-bg: rgba(226,232,240,0.12);
        --status-ok-bg: rgba(22,163,74,0.18);
        --status-ok-ink: #bbf7d0;
        --status-bad-bg: rgba(239,68,68,0.18);
        --status-bad-ink: #fecaca;
        --shadow: rgba(2,6,23,0.45);
      }
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: "Segoe UI", "Helvetica Neue", sans-serif;
      color: var(--ink);
      background: radial-gradient(circle at 10% 20%, var(--glow-a) 0%, transparent 35%),
                  radial-gradient(circle at 90% 10%, var(--glow-b) 0%, transparent 30%),
                  linear-gradient(140deg, var(--bg-a), var(--bg-b));
      display: grid;
      place-items: center;
      padding: 24px;
    }
    .card {
      width: min(720px, 100%);
      background: var(--panel);
      border-radius: 18px;
      padding: 24px;
      box-shadow: 0 20px 60px var(--shadow);
      animation: rise .5s ease;
      backdrop-filter: blur(14px);
    }
    @keyframes rise {
      from { opacity: 0; transform: translateY(10px); }
      to { opacity: 1; transform: translateY(0); }
    }
    h1 { margin-top: 0; }
    p { color: var(--muted); }
    .row { display: flex; gap: 10px; flex-wrap: wrap; }
    input {
      flex: 1;
      min-width: 220px;
      border: 1px solid var(--field-border);
      border-radius: 10px;
      padding: 12px;
      font-size: 16px;
      color: var(--ink);
      background: var(--field-bg);
    }
    input::placeholder { color: var(--muted); }
    button {
      border: 0;
      border-radius: 10px;
      padding: 12px 16px;
      font-size: 15px;
      font-weight: 600;
      cursor: pointer;
      background: var(--accent);
      color: white;
      transition: transform .12s ease, filter .12s ease;
    }
    button:hover { transform: translateY(-1px); filter: brightness(1.05); }
    .secondary { background: var(--secondary); }
    .status {
      margin-top: 16px;
      border-radius: 10px;
      padding: 12px;
      background: var(--status-bg);
    }
    .ok { background: var(--status-ok-bg); color: var(--status-ok-ink); }
    .bad { background: var(--status-bad-bg); color: var(--status-bad-ink); }
    .meta { margin-top: 10px; font-size: 14px; color: var(--muted); }
  </style>
</head>
<body>
  <main class="card">
    <h1>PulseOrPerish</h1>
    <p>Submit a proof of life before the deadline.</p>
    <div class="row">
      <input id="pwd" type="password" placeholder="Password" />
      <button id="alive">Proof of life</button>
      <button class="secondary" id="refresh">Refresh status</button>
    </div>
    <div id="status" class="status">Waiting...</div>
    <div class="meta" id="meta"></div>
  </main>
  <script>
    const statusBox = document.getElementById('status');
    const meta = document.getElementById('meta');
    const pwdInput = document.getElementById('pwd');

    function formatDate(isoString) {
      if (!isoString) return '-';
      const date = new Date(isoString);
      return date.toLocaleString('fr-FR', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    }

    function formatDuration(durationStr) {
      if (!durationStr) return '-';

      // Parse Go duration format: 2h30m15s
      const parts = [];
      let match;
      const regex = /(\d+)([a-z]+)/g;

      let months = 0, days = 0, hours = 0, minutes = 0, seconds = 0;

      while ((match = regex.exec(durationStr)) !== null) {
        const value = parseInt(match[1]);
        const unit = match[2];

        switch (unit) {
          case 'h': hours = value; break;
          case 'm': minutes = value; break;
          case 's': seconds = value; break;
        }
      }

      // Convert to days and remaining hours
      if (hours >= 24) {
        days = Math.floor(hours / 24);
        hours = hours % 24;
      }

      if (days > 0) parts.push(days + ' day' + (days > 1 ? 's' : ''));
      if (hours > 0) parts.push(hours + ' hour' + (hours > 1 ? 's' : ''));
      if (minutes > 0) parts.push(minutes + ' minute' + (minutes > 1 ? 's' : ''));

      return parts.length > 0 ? parts.join(', ') : 'less than a minute';
    }

    function render(data) {
      const overdue = !!data.overdue;
      statusBox.className = 'status ' + (overdue ? 'bad' : 'ok');
      statusBox.textContent = overdue ? 'DEADLINE EXCEEDED' : 'ok';
      meta.innerHTML =
        'Dry-run mode: ' + (data.dryRun ? 'yes' : 'no') + '<br/>' +
        'Last proof:   ' + formatDate(data.lastProofAt) + '<br/>' +
        'Deletion:     ' + formatDate(data.nextDeletion) + '<br/>' +
        'Time remaining: ' + formatDuration(data.timeRemaining);
    }

    async function refresh() {
      const res = await fetch('/status', { headers: { 'Accept': 'application/json' } });
      const data = await res.json();
      if (!res.ok) {
        statusBox.className = 'status bad';
        statusBox.textContent = data.error || 'Error';
        meta.textContent = '';
        return;
      }
      render(data);
    }

    async function alive() {
      const res = await fetch('/alive', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
        body: JSON.stringify({ password: pwdInput.value || '' })
      });
      const data = await res.json();
      if (!res.ok) {
        statusBox.className = 'status bad';
        statusBox.textContent = data.error || 'Error';
        return;
      }
      await refresh();
    }

    document.getElementById('alive').addEventListener('click', alive);
    document.getElementById('refresh').addEventListener('click', refresh);
    setInterval(refresh, 15000);
    refresh();
  </script>
</body>
</html>`
