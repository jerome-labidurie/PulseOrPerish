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
    r.Post("/alive", s.handleAlive)
	})

	return r
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
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
	if !s.authenticateRequest(r) {
		s.log.Warn().Str("remote", r.RemoteAddr).Msg("authentication failed")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
		return
	}

	at, err := s.monitor.RegisterProof("http")
	if err != nil {
		s.log.Error().Err(err).Msg("failed to persist proof")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist proof"})
		return
	}
	status := s.monitor.Snapshot(time.Now().UTC())
	writeJSON(w, http.StatusOK, map[string]any{
		"lastProofAt":   at,
		"nextDeletion":  status.NextDeletion,
		"timeRemaining": status.TimeRemaining,
		"dryRun":        status.DryRun,
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
	st := s.monitor.Snapshot(time.Now().UTC())
	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Path == "/status" {
		writeJSON(w, http.StatusOK, st)
		return
	}
	writeJSON(w, http.StatusOK, st)
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
      --bg-a: #0f172a;
      --bg-b: #1d4ed8;
      --panel: rgba(255,255,255,0.92);
      --ink: #0b1320;
      --accent: #ef4444;
      --ok: #16a34a;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: "Segoe UI", "Helvetica Neue", sans-serif;
      color: var(--ink);
      background: radial-gradient(circle at 10% 20%, #93c5fd 0%, transparent 35%),
                  radial-gradient(circle at 90% 10%, #fca5a5 0%, transparent 30%),
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
      box-shadow: 0 20px 60px rgba(0,0,0,0.3);
      animation: rise .5s ease;
    }
    @keyframes rise {
      from { opacity: 0; transform: translateY(10px); }
      to { opacity: 1; transform: translateY(0); }
    }
    h1 { margin-top: 0; }
    .row { display: flex; gap: 10px; flex-wrap: wrap; }
    input {
      flex: 1;
      min-width: 220px;
      border: 1px solid #d1d5db;
      border-radius: 10px;
      padding: 12px;
      font-size: 16px;
    }
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
    .secondary { background: #0ea5e9; }
    .status {
      margin-top: 16px;
      border-radius: 10px;
      padding: 12px;
      background: #e5e7eb;
    }
    .ok { background: #dcfce7; color: #14532d; }
    .bad { background: #fee2e2; color: #7f1d1d; }
    .meta { margin-top: 10px; font-size: 14px; opacity: 0.9; }
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
