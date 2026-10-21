package observe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Heartbeats pings Better Stack's heartbeat URL for a job after it runs, so a job that stops
// running raises an alert. Each name is pinged at most once a minute, with a short timeout,
// and a failing ping is logged at most once an hour.
type Heartbeats struct {
	URLs   map[string]string
	Client *http.Client // default: 2 second timeout
	Logger *slog.Logger
	Now    func() time.Time

	mu       sync.Mutex
	pinged   map[string]time.Time
	reported map[string]time.Time
}

// ParseHeartbeats reads "name=url" pairs separated by commas.
func ParseHeartbeats(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, u, ok := strings.Cut(pair, "=")
		if !ok || name == "" || !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			return nil, fmt.Errorf("heartbeat %q must be name=https://...", pair)
		}
		out[strings.TrimSpace(name)] = strings.TrimSpace(u)
	}
	return out, nil
}

// Ping tells the heartbeat for name that its job ran. It does nothing for a name with no URL.
func (h *Heartbeats) Ping(ctx context.Context, name string) {
	if h == nil {
		return
	}
	u, ok := h.URLs[name]
	if !ok || !h.due(&h.pinged, name, time.Minute) {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err == nil {
		var res *http.Response
		if res, err = h.client().Do(req); err == nil {
			_ = res.Body.Close()
			if res.StatusCode >= 300 {
				err = fmt.Errorf("status %d", res.StatusCode)
			}
		}
	}
	if err != nil && h.due(&h.reported, name, time.Hour) {
		h.log().Warn("heartbeat not sent", "heartbeat", name, "error", err)
	}
}

func (h *Heartbeats) due(seen *map[string]time.Time, name string, every time.Duration) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if *seen == nil {
		*seen = map[string]time.Time{}
	}
	if last, ok := (*seen)[name]; ok && now.Sub(last) < every {
		return false
	}
	(*seen)[name] = now
	return true
}

func (h *Heartbeats) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 2 * time.Second}
}

func (h *Heartbeats) log() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}
