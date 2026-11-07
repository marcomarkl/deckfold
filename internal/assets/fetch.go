package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher downloads http(s) resources during conversion.
type Fetcher struct {
	Client   *http.Client
	MaxBytes int64
	Hosts    map[string]bool // hosts contacted, for the report
	cache    map[string]fetched
	// AllowPrivate allows private addresses (tests only).
	AllowPrivate bool
}

type fetched struct {
	data []byte
	mime string
	err  error
}

// ErrPrivateAddr reports a target in the local or private network.
var ErrPrivateAddr = errors.New("target is in the local or private network and is not downloaded")

// UserAgent of a current browser, so Google Fonts serves woff2.
const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

// NewFetcher creates a fetcher. proxy is optional (--proxy); otherwise HTTPS_PROXY/NO_PROXY apply.
func NewFetcher(proxy string, timeout time.Duration, maxBytes int64) (*Fetcher, error) {
	f := &Fetcher{MaxBytes: maxBytes, Hosts: map[string]bool{}, cache: map[string]fetched{}}
	proxyFn := http.ProxyFromEnvironment
	if proxy != "" {
		pu, err := url.Parse(proxy)
		if err != nil || pu.Host == "" {
			return nil, fmt.Errorf("--proxy: invalid URL %q", proxy)
		}
		proxyFn = http.ProxyURL(pu)
	}
	dialer := &net.Dialer{Timeout: timeout}
	tr := &http.Transport{
		Proxy: proxyFn,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error = ErrPrivateAddr
			for _, ip := range ips {
				if !f.AllowPrivate && isBlockedIP(ip.IP) && !isProxyHost(host, proxyFn) {
					continue
				}
				c, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err == nil {
					return c, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
	}
	f.Client = &http.Client{Transport: tr, Timeout: 4 * timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
				return errors.New("redirect to a disallowed scheme")
			}
			return nil
		}}
	return f, nil
}

// isProxyHost allows the connection to the configured proxy, even if it is in a private network.
func isProxyHost(host string, proxyFn func(*http.Request) (*url.URL, error)) bool {
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	if pu, err := proxyFn(req); err == nil && pu != nil {
		return pu.Hostname() == host
	}
	return false
}

// isBlockedIP blocks loopback, private, link-local, unspecified and multicast addresses.
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// CGNAT 100.64.0.0/10 and 0.0.0.0/8
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 0 || (v4[0] == 100 && v4[1]&0xc0 == 64) {
			return true
		}
	}
	return false
}

// Get downloads a URL (cached) and returns its data and MIME type.
func (f *Fetcher) Get(raw string) ([]byte, string, error) {
	if c, ok := f.cache[raw]; ok {
		return c.data, c.mime, c.err
	}
	data, mime, err := f.get(raw)
	f.cache[raw] = fetched{data, mime, err}
	return data, mime, err
}

func (f *Fetcher) get(raw string) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", fmt.Errorf("scheme %q is not downloaded", u.Scheme)
	}
	if !f.AllowPrivate {
		// Pre-check, so requests through a proxy cannot reach an internal target either.
		ips, err := net.LookupIP(u.Hostname())
		if err != nil {
			return nil, "", err
		}
		for _, ip := range ips {
			if isBlockedIP(ip) {
				return nil, "", ErrPrivateAddr
			}
		}
	}
	f.Hosts[u.Hostname()] = true
	req, err := http.NewRequest("GET", raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, f.MaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > f.MaxBytes {
		return nil, "", fmt.Errorf("larger than the limit of %d MB", f.MaxBytes>>20)
	}
	mime := resp.Header.Get("Content-Type")
	if i := strings.Index(mime, ";"); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	return data, mime, nil
}
