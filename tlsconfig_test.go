package SherryServer

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
)

func tlsTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TLSAutoEnabled", "TLSAutoDomains", "TLSAutoCacheDir", "TLSAutoAcceptTOS", "TLSAutoEmail", "TLSAutoDirectoryURL", "sslCertification", "sslKeyfile"} {
		t.Setenv(key, "")
	}
}

func TestTLSSettings(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		mode    string
		wantErr string
	}{
		{"default HTTP", nil, "http", ""},
		{"disabled ignores auto settings", map[string]string{"TLSAutoEnabled": "false", "TLSAutoDomains": "bad", "TLSAutoAcceptTOS": "bad", "TLSAutoCacheDir": "unused"}, "http", ""},
		{"legacy partial cert", map[string]string{"sslCertification": "cert.pem"}, "http", ""},
		{"legacy partial key", map[string]string{"sslKeyfile": "key.pem"}, "http", ""},
		{"legacy HTTPS", map[string]string{"sslKeyfile": "key.pem", "sslCertification": "cert.pem"}, "file-tls", ""},
		{"boolean", map[string]string{"TLSAutoEnabled": "perhaps"}, "", "TLSAutoEnabled"},
		{"conflicting cert", map[string]string{"TLSAutoEnabled": "true", "sslCertification": "cert.pem"}, "", "conflicts"},
		{"conflicting key", map[string]string{"TLSAutoEnabled": "true", "sslKeyfile": "key.pem"}, "", "conflicts"},
		{"no domains", map[string]string{"TLSAutoEnabled": "true"}, "", "TLSAutoDomains"},
		{"URL domain", map[string]string{"TLSAutoEnabled": "true", "TLSAutoDomains": "https://example.com"}, "", "TLSAutoDomains"},
		{"no cache", map[string]string{"TLSAutoEnabled": "true", "TLSAutoDomains": "example.com"}, "", "TLSAutoCacheDir"},
		{"no consent", map[string]string{"TLSAutoEnabled": "true", "TLSAutoDomains": "example.com", "TLSAutoCacheDir": "unused"}, "", "TLSAutoAcceptTOS"},
		{"bad consent", map[string]string{"TLSAutoEnabled": "true", "TLSAutoDomains": "example.com", "TLSAutoCacheDir": "unused", "TLSAutoAcceptTOS": "maybe"}, "", "TLSAutoAcceptTOS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tlsTestEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			c, err := tlsSettingsFromEnv()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v, want %s", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			p, err := newServerTLS(c)
			if err != nil {
				t.Fatal(err)
			}
			if p.mode() != tc.mode || p.manager != nil {
				t.Fatalf("unexpected plan: %+v", p)
			}
		})
	}
}

func TestAutoTLSSettings(t *testing.T) {
	tlsTestEnv(t)
	t.Setenv("TLSAutoEnabled", "true")
	t.Setenv("TLSAutoDomains", " Example.COM.,example.com,例子.台灣 ")
	t.Setenv("TLSAutoCacheDir", filepath.Join(t.TempDir(), "cache"))
	t.Setenv("TLSAutoAcceptTOS", "true")
	t.Setenv("TLSAutoEmail", "admin@example.com")
	c, err := tlsSettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(c.domains, []string{"example.com", "xn--fsqu00a.xn--kpry57d"}) {
		t.Fatal(c.domains)
	}
	p, err := newServerTLS(c)
	if err != nil {
		t.Fatal(err)
	}
	if p.mode() != "auto-tls" || p.manager.Email != "admin@example.com" || !p.manager.Prompt("terms") {
		t.Fatal("manager configuration")
	}
	c.directoryURL = "https://staging.example.com/directory"
	other, err := newServerTLS(c)
	if err != nil {
		t.Fatal(err)
	}
	if p.manager.Cache == other.manager.Cache {
		t.Fatal("CA caches must be isolated")
	}
	for _, invalid := range []string{"http://example.com/directory", "https:///missing", "/directory", "https://u:p@example.com", "https://example.com/#secret", "https://example.com:bad/"} {
		t.Setenv("TLSAutoDirectoryURL", invalid)
		if _, err := tlsSettingsFromEnv(); err == nil {
			t.Fatalf("accepted %q", invalid)
		}
	}
}

func TestTLSDomains(t *testing.T) {
	for _, name := range []string{"", "localhost", "127.0.0.1", "::1", "*.example.com", "https://example.com", "example.com:443", "a..com", "-a.com", "a-.com", "a_b.com", "example.com/path", " example.com", "example.com..", strings.Repeat("a", 64) + ".com"} {
		if _, err := normalizeTLSDomain(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestTLSCacheErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(file, "child")} {
		if err := checkTLSCacheDir(path); err == nil || !strings.Contains(err.Error(), "TLSAutoCacheDir") {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if runtime.GOOS != "windows" {
		wide := filepath.Join(t.TempDir(), "wide")
		if err := os.Mkdir(wide, 0755); err != nil {
			t.Fatal(err)
		}
		if err := checkTLSCacheDir(wide); err == nil {
			t.Fatal("accepted public cache directory")
		}
	}
}

func TestAutoTLSMerge(t *testing.T) {
	c := tlsSettings{auto: true, domains: []string{"example.com"}, cacheDir: t.TempDir(), directoryURL: "https://ca.example/directory"}
	p, err := newServerTLS(c)
	if err != nil {
		t.Fatal(err)
	}
	base := &tls.Config{MinVersion: tls.VersionTLS13, NextProtos: []string{"custom"}, SessionTicketsDisabled: true}
	srv := &http.Server{TLSConfig: base}
	if err := p.configure(srv, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if srv.TLSConfig == base || !srv.TLSConfig.SessionTicketsDisabled || srv.TLSConfig.MinVersion != tls.VersionTLS13 {
		t.Fatal("caller settings lost")
	}
	if !slices.Equal(base.NextProtos, []string{"custom"}) {
		t.Fatal("mutated caller ALPN")
	}
	for _, proto := range []string{"custom", "h2", "http/1.1", acme.ALPNProto} {
		if !slices.Contains(srv.TLSConfig.NextProtos, proto) {
			t.Fatal("missing", proto)
		}
	}
	for _, bad := range []*tls.Config{
		{Certificates: []tls.Certificate{{}}},
		{NameToCertificate: map[string]*tls.Certificate{"example.com": {}}},
		{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, nil }},
		{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) { return nil, nil }},
		{ClientAuth: tls.RequireAnyClientCert}, {MinVersion: tls.VersionTLS10}, {MaxVersion: tls.VersionTLS11},
		{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS12},
	} {
		s := &http.Server{TLSConfig: bad}
		if err := p.configure(s, nil); err == nil {
			t.Fatalf("accepted conflicting config %+v", bad)
		}
		if s.TLSConfig != bad {
			t.Fatal("mutated config on failure")
		}
	}
}

func TestNewServerTLSValidationAndSnapshot(t *testing.T) {
	tlsTestEnv(t)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("envfile", []byte("SystemName=test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SessionCookieAuthKey", strings.Repeat("x", 32))
	t.Setenv("SessionCookieEncryptionKey", "")
	t.Setenv("TLSAutoEnabled", "true")
	t.Setenv("sslKeyfile", "conflict")
	if _, err := NewServer(":8080", "", ""); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatal(err)
	}
	t.Setenv("sslKeyfile", "")
	t.Setenv("TLSAutoDomains", "example.com")
	t.Setenv("TLSAutoAcceptTOS", "true")
	t.Setenv("TLSAutoCacheDir", filepath.Join(t.TempDir(), "cache"))
	srv, err := NewServer(":8080", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLSAutoEnabled", "false")
	p, err := srv.prepareTLS()
	if err != nil || p.mode() != "auto-tls" || srv.Server.Addr != ":8080" {
		t.Fatalf("snapshot: %v", err)
	}
}

// startTLSListener exercises the same serve branch used by Start, on a loopback port.
func startTLSListener(t *testing.T, p *serverTLS, handlers ...http.Handler) (*http.Server, string) {
	t.Helper()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }), ReadHeaderTimeout: 5 * time.Second}
	if len(handlers) > 0 {
		srv.Handler = handlers[0]
	}
	if err := p.configure(srv, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.serve(srv, ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			t.Error(err)
		}
		if err := <-done; err != http.ErrServerClosed {
			t.Errorf("serve: %v", err)
		}
	})
	return srv, ln.Addr().String()
}
