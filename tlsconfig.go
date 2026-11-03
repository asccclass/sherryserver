package SherryServer

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/idna"
)

// tlsSettings is a snapshot: changing the environment after NewServer has no effect.
type tlsSettings struct {
	auto         bool
	domains      []string
	cacheDir     string
	email        string
	directoryURL string
	certFile     string
	keyFile      string
}

func tlsSettingsFromEnv() (tlsSettings, error) {
	c := tlsSettings{certFile: os.Getenv("sslCertification"), keyFile: os.Getenv("sslKeyfile")}
	enabled := strings.TrimSpace(os.Getenv("TLSAutoEnabled"))
	if enabled != "" {
		var err error
		c.auto, err = strconv.ParseBool(enabled)
		if err != nil {
			return c, fmt.Errorf("TLSAutoEnabled must be a boolean")
		}
	}
	if !c.auto {
		return c, nil
	}
	if c.certFile != "" || c.keyFile != "" {
		return c, fmt.Errorf("TLSAutoEnabled conflicts with sslCertification/sslKeyfile")
	}
	for _, raw := range strings.Split(os.Getenv("TLSAutoDomains"), ",") {
		domain, err := normalizeTLSDomain(strings.TrimSpace(raw))
		if err != nil {
			return c, fmt.Errorf("TLSAutoDomains: %w", err)
		}
		if !slices.Contains(c.domains, domain) {
			c.domains = append(c.domains, domain)
		}
	}
	c.cacheDir = strings.TrimSpace(os.Getenv("TLSAutoCacheDir"))
	if c.cacheDir == "" {
		return c, fmt.Errorf("TLSAutoCacheDir is required when TLSAutoEnabled=true")
	}
	accepted, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("TLSAutoAcceptTOS")))
	if err != nil || !accepted {
		return c, fmt.Errorf("TLSAutoAcceptTOS must be true when TLSAutoEnabled=true")
	}
	c.email = strings.TrimSpace(os.Getenv("TLSAutoEmail"))
	c.directoryURL = strings.TrimSpace(os.Getenv("TLSAutoDirectoryURL"))
	if c.directoryURL == "" {
		c.directoryURL = autocert.DefaultACMEDirectory
	}
	u, err := url.Parse(c.directoryURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return c, fmt.Errorf("TLSAutoDirectoryURL must be an absolute HTTPS URL without credentials or fragment")
	}
	return c, nil
}

func normalizeTLSDomain(raw string) (string, error) {
	name, err := idna.Lookup.ToASCII(raw)
	if err != nil {
		return "", fmt.Errorf("invalid domain %q", raw)
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if len(name) > 253 || !strings.Contains(name, ".") || net.ParseIP(name) != nil {
		return "", fmt.Errorf("invalid domain %q: expected a DNS name", raw)
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid domain %q", raw)
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return "", fmt.Errorf("invalid domain %q", raw)
			}
		}
	}
	return name, nil
}

type serverTLS struct {
	settings tlsSettings
	manager  *autocert.Manager
}

func newServerTLS(c tlsSettings) (*serverTLS, error) {
	p := &serverTLS{settings: c}
	if !c.auto {
		return p, nil
	}
	// Separate account keys and certificates for each CA, especially staging vs production.
	namespace := fmt.Sprintf("%x", sha256.Sum256([]byte(c.directoryURL)))
	cacheDir := filepath.Join(c.cacheDir, namespace)
	if err := checkTLSCacheDir(c.cacheDir); err != nil {
		return nil, err
	}
	if err := checkTLSCacheDir(cacheDir); err != nil {
		return nil, err
	}
	p.manager = &autocert.Manager{
		Cache:      autocert.DirCache(cacheDir),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(c.domains...),
		Email:      c.email,
		Client:     &acme.Client{DirectoryURL: c.directoryURL},
	}
	return p, nil
}

func checkTLSCacheDir(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("TLSAutoCacheDir: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("TLSAutoCacheDir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("TLSAutoCacheDir must be a directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("TLSAutoCacheDir permissions must restrict access to the service account (0700)")
	}
	f, err := os.CreateTemp(dir, ".access-check-*")
	if err != nil {
		return fmt.Errorf("TLSAutoCacheDir is not writable: %w", err)
	}
	defer os.Remove(f.Name())
	_, writeErr := f.WriteString("cache-access-check")
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("TLSAutoCacheDir write check: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("TLSAutoCacheDir close check: %w", closeErr)
	}
	if data, err := os.ReadFile(f.Name()); err != nil || string(data) != "cache-access-check" {
		return fmt.Errorf("TLSAutoCacheDir read check failed")
	}
	return nil
}

func (p *serverTLS) mode() string {
	if p.settings.auto {
		return "auto-tls"
	}
	if p.settings.certFile != "" && p.settings.keyFile != "" {
		return "file-tls"
	}
	return "http"
}

// configure clones caller configuration, preserving TLS settings that do not bypass
// certificate selection or require a client certificate during an ACME challenge.
func (p *serverTLS) configure(srv *http.Server, logger *zap.Logger) error {
	if !p.settings.auto {
		return nil
	}
	cfg := p.manager.TLSConfig()
	if base := srv.TLSConfig; base != nil {
		if len(base.Certificates) != 0 || len(base.NameToCertificate) != 0 || base.GetCertificate != nil || base.GetConfigForClient != nil {
			return fmt.Errorf("TLSAutoEnabled conflicts with TLSConfig certificate providers")
		}
		if base.ClientAuth != tls.NoClientCert {
			return fmt.Errorf("TLSAutoEnabled conflicts with TLSConfig.ClientAuth (ACME requires no client certificate)")
		}
		if base.MaxVersion != 0 && base.MaxVersion < tls.VersionTLS12 {
			return fmt.Errorf("TLSAutoEnabled requires TLSConfig.MaxVersion >= TLS 1.2")
		}
		cfg = base.Clone()
		cfg.NextProtos = slices.Clone(base.NextProtos)
		for _, proto := range p.manager.TLSConfig().NextProtos {
			if !slices.Contains(cfg.NextProtos, proto) {
				cfg.NextProtos = append(cfg.NextProtos, proto)
			}
		}
	}
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS12
	}
	if cfg.MinVersion < tls.VersionTLS12 || cfg.MinVersion > tls.VersionTLS13 || cfg.MaxVersion != 0 && cfg.MaxVersion < cfg.MinVersion {
		return fmt.Errorf("TLSAutoEnabled requires a valid TLSConfig version range within TLS 1.2 or newer")
	}
	policy := autocert.HostWhitelist(p.settings.domains...)
	cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		name, err := normalizeTLSDomain(hello.ServerName)
		if err == nil {
			err = policy(context.Background(), name)
		}
		if err != nil {
			if logger != nil {
				logger.Warn("Auto TLS rejected SNI")
			}
			return nil, fmt.Errorf("auto TLS: SNI is missing, invalid or not allowed")
		}
		// Do not modify the TLS stack's ClientHelloInfo; it can be shared with other hooks.
		normalized := *hello
		normalized.ServerName = name
		cert, err := p.manager.GetCertificate(&normalized)
		if err != nil && logger != nil {
			// Do not print CA response bodies or account data from remote errors.
			logger.Warn("Auto TLS certificate unavailable", zap.String("domain", name))
		}
		if err != nil {
			return nil, fmt.Errorf("auto TLS: certificate unavailable for %s", name)
		}
		return cert, err
	}
	srv.TLSConfig = cfg
	return nil
}

// serve is shared by Start and local listener tests; no extra challenge server is used.
func (p *serverTLS) serve(srv *http.Server, listener net.Listener) error {
	switch p.mode() {
	case "auto-tls":
		return srv.ServeTLS(listener, "", "")
	case "file-tls":
		return srv.ServeTLS(listener, p.settings.certFile, p.settings.keyFile)
	default:
		return srv.Serve(listener)
	}
}

func (app *Server) prepareTLS() (*serverTLS, error) {
	p := app.tls
	if p == nil { // Keep manually constructed Server values usable.
		c, err := tlsSettingsFromEnv()
		if err != nil {
			return nil, err
		}
		p, err = newServerTLS(c)
		if err != nil {
			return nil, err
		}
	}
	if err := p.configure(app.Server, app.Logger); err != nil {
		return nil, err
	}
	return p, nil
}
