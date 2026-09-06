package sim

import (
	"bytes"
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// secretHeader carries the world's link secret between its own machines:
// the core and its regions and distribution systems share SKYD_LINK_SECRET,
// and every control-plane call between them presents it. The same secret
// keys the switch links' tokens, so one secret is the world's.
const secretHeader = "X-Skyd-Secret"

// secretOK says whether a request carries the world's secret.
func (s *Sim) secretOK(r *http.Request) bool {
	got := r.Header.Get(secretHeader)
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(s.linkSecret)) == 1
}

// requireSecret guards a handler that only this world's own machines may
// call: registration, tokens, saved state, the feeds between shards.
func (s *Sim) requireSecret(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.secretOK(r) {
			http.Error(w, "this is the world's own control plane; it needs the link secret", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// fedPost is a POST between this world's machines, carrying the secret.
func (s *Sim) fedPost(client *http.Client, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(secretHeader, s.linkSecret)
	return client.Do(req)
}

// noRedirects is a client policy for URLs strangers supplied: a redirect
// would let a public host hand the request on to a private one.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// peerClient is the client for a URL another world or a player gave us:
// bounded in time, and it follows nothing.
func peerClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: noRedirects}
}

// privateIP says whether an address is one the internet cannot reach:
// loopback, RFC 1918 and ULA (Fly's 6PN is fdaa::/16), link-local, the
// carrier-grade NAT range, multicast and the unspecified address. A
// server that fetches what a stranger names must not be turned on the
// network behind it.
func privateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 0 || (v4[0] == 100 && v4[1]&0xc0 == 64) || (v4[0] == 192 && v4[1] == 0 && v4[2] == 0)
	}
	return false
}

// publicHost resolves a host and refuses one whose addresses are not all
// public, unless the world was started with -allow-private-peers (a test
// or a gate on one machine).
func (s *Sim) publicHost(ctx context.Context, host string) error {
	if s.allowPrivate {
		return nil
	}
	if host == "" {
		return fmt.Errorf("no host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("%s is not a public host", host)
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(rctx, host)
		if err != nil {
			return fmt.Errorf("%s does not resolve: %w", host, err)
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}
	if len(ips) == 0 {
		return fmt.Errorf("%s has no address", host)
	}
	for _, ip := range ips {
		if privateIP(ip) {
			return fmt.Errorf("%s is %s, which the internet cannot reach", host, ip)
		}
	}
	return nil
}

// checkedURL checks a URL a stranger supplied -- another world's, a
// player's node's -- and returns it trimmed: http or https, a public host,
// nothing else.
func (s *Sim) checkedURL(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("not a URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("a node's URL is http or https, not %q", u.Scheme)
	}
	if u.User != nil {
		return "", fmt.Errorf("a node's URL carries no credentials")
	}
	if err := s.publicHost(ctx, u.Hostname()); err != nil {
		return "", err
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// publicHostPort checks a host:port another world gave for its switch.
func (s *Sim) publicHostPort(ctx context.Context, addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("a switch address is host:port: %w", err)
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return fmt.Errorf("bad port %q", port)
	}
	return s.publicHost(ctx, host)
}

// clientIP is who is asking, through Fly's proxy or not.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipLimiter allows one action per interval per address, remembering at
// most a bounded number of addresses so the memory of strangers is itself
// bounded.
type ipLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	max      int
	last     map[string]time.Time
}

func newIPLimiter(interval time.Duration, max int) *ipLimiter {
	return &ipLimiter{interval: interval, max: max, last: map[string]time.Time{}}
}

// allow says whether the address may act now, and records it if so.
func (l *ipLimiter) allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, ok := l.last[ip]; ok && now.Sub(t) < l.interval {
		return false
	}
	if len(l.last) >= l.max {
		for k, t := range l.last {
			if now.Sub(t) >= l.interval {
				delete(l.last, k)
			}
		}
		if len(l.last) >= l.max {
			return false
		}
	}
	l.last[ip] = now
	return true
}
