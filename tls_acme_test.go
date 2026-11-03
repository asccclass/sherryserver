package SherryServer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/crypto/acme"
)

// mockACME implements only the ACME v2 operations exercised by autocert. It
// validates TLS-ALPN-01 over an actual socket, checks the CSR signature and signs
// a leaf with a private test CA. It is not a general-purpose ACME server.
type mockACME struct {
	server        *httptest.Server
	root          *x509.Certificate
	key           *ecdsa.PrivateKey
	roots         *x509.CertPool
	mu            sync.Mutex
	orders        []*mockOrder
	addr          string
	thumb         string
	issued        int
	firstLifetime time.Duration
	failOrders    atomic.Bool
	orderAttempts atomic.Int64
	challenges    atomic.Int64
}

type mockOrder struct {
	domain string
	status string
	cert   []byte
}

func newMockACME(t *testing.T, lifetime time.Duration) *mockACME {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{SerialNumber: big.NewInt(1000), Subject: pkix.Name{CommonName: "Local TLS test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	ca := &mockACME{root: root, key: key, roots: x509.NewCertPool(), firstLifetime: lifetime}
	ca.roots.AddCert(root)
	ca.server = httptest.NewTLSServer(http.HandlerFunc(ca.handle))
	t.Cleanup(ca.server.Close)
	return ca
}

func (ca *mockACME) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Replay-Nonce", "local-test-nonce")
	w.Header().Set("Content-Type", "application/json")
	url := ca.server.URL
	if r.URL.Path == "/directory" {
		json.NewEncoder(w).Encode(map[string]any{"newNonce": url + "/nonce", "newAccount": url + "/account", "newOrder": url + "/new-order", "meta": map[string]string{"termsOfService": url + "/terms"}})
		return
	}
	if r.URL.Path == "/nonce" {
		return
	}
	var jws struct{ Protected, Payload string }
	if err := json.NewDecoder(r.Body).Decode(&jws); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	payload, _ := base64.RawURLEncoding.DecodeString(jws.Payload)
	ca.mu.Lock()
	defer ca.mu.Unlock()
	if r.URL.Path == "/account" {
		var header struct{ JWK map[string]string }
		protected, _ := base64.RawURLEncoding.DecodeString(jws.Protected)
		json.Unmarshal(protected, &header)
		canonical, _ := json.Marshal(header.JWK)
		sum := sha256.Sum256(canonical)
		ca.thumb = base64.RawURLEncoding.EncodeToString(sum[:])
		w.Header().Set("Location", url+"/account/1")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status":"valid"}`)
		return
	}
	if r.URL.Path == "/new-order" {
		ca.orderAttempts.Add(1)
		if ca.failOrders.Load() {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"type":"urn:ietf:params:acme:error:rejectedIdentifier","detail":"REMOTE_SECRET_MARKER"}`)
			return
		}
		var req struct{ Identifiers []struct{ Value string } }
		if json.Unmarshal(payload, &req) != nil || len(req.Identifiers) != 1 {
			http.Error(w, "bad identifiers", 400)
			return
		}
		id := len(ca.orders)
		ca.orders = append(ca.orders, &mockOrder{domain: req.Identifiers[0].Value, status: "pending"})
		w.Header().Set("Location", fmt.Sprintf("%s/order/%d", url, id))
		w.WriteHeader(http.StatusCreated)
		ca.writeOrder(w, id)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 {
		http.Error(w, "unknown endpoint", 404)
		return
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id < 0 || id >= len(ca.orders) {
		http.Error(w, "unknown order", 404)
		return
	}
	o := ca.orders[id]
	switch parts[0] {
	case "order":
		ca.writeOrder(w, id)
	case "authz":
		status := "pending"
		if o.status == "ready" || o.status == "valid" {
			status = "valid"
		}
		json.NewEncoder(w).Encode(map[string]any{"status": status, "identifier": map[string]string{"type": "dns", "value": o.domain}, "challenges": []map[string]string{{"type": "tls-alpn-01", "url": fmt.Sprintf("%s/challenge/%d", url, id), "token": fmt.Sprintf("token-%d", id)}}})
	case "challenge":
		if err := ca.validateChallenge(o.domain, fmt.Sprintf("token-%d", id)); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		ca.challenges.Add(1)
		o.status = "ready"
		json.NewEncoder(w).Encode(map[string]string{"type": "tls-alpn-01", "status": "valid", "url": url + r.URL.Path})
	case "finalize":
		if o.status != "ready" {
			http.Error(w, "order not ready", 400)
			return
		}
		var req struct{ CSR string }
		json.Unmarshal(payload, &req)
		der, _ := base64.RawURLEncoding.DecodeString(req.CSR)
		csr, err := x509.ParseCertificateRequest(der)
		if err != nil || csr.CheckSignature() != nil || len(csr.DNSNames) != 1 || csr.DNSNames[0] != o.domain {
			http.Error(w, "invalid CSR", 400)
			return
		}
		ca.issued++
		lifetime := 12 * time.Hour
		if ca.issued == 1 && ca.firstLifetime > 0 {
			lifetime = ca.firstLifetime
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(int64(ca.issued)), DNSNames: csr.DNSNames, NotBefore: time.Now().Add(-time.Second), NotAfter: time.Now().Add(lifetime), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		cert, err := x509.CreateCertificate(rand.Reader, leaf, ca.root, csr.PublicKey, ca.key)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		o.cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
		o.status = "valid"
		ca.writeOrder(w, id)
	case "cert":
		w.Header().Set("Content-Type", "application/pem-certificate-chain")
		w.Write(o.cert)
		pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: ca.root.Raw})
	default:
		http.Error(w, "unknown endpoint", 404)
	}
}

// ca.mu must be held by the caller.
func (ca *mockACME) writeOrder(w http.ResponseWriter, id int) {
	o := ca.orders[id]
	url := ca.server.URL
	json.NewEncoder(w).Encode(map[string]any{"status": o.status, "authorizations": []string{fmt.Sprintf("%s/authz/%d", url, id)}, "finalize": fmt.Sprintf("%s/finalize/%d", url, id), "certificate": fmt.Sprintf("%s/cert/%d", url, id)})
}

func (ca *mockACME) validateChallenge(domain, token string) error {
	c, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", ca.addr, &tls.Config{ServerName: domain, NextProtos: []string{acme.ALPNProto}, InsecureSkipVerify: true}) // challenge certs are intentionally self-signed
	if err != nil {
		return err
	}
	defer c.Close()
	state := c.ConnectionState()
	if state.NegotiatedProtocol != acme.ALPNProto || len(state.PeerCertificates) != 1 {
		return fmt.Errorf("invalid challenge negotiation")
	}
	cert := state.PeerCertificates[0]
	if err := cert.VerifyHostname(domain); err != nil {
		return err
	}
	want := sha256.Sum256([]byte(token + "." + ca.thumb))
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 31}) && ext.Critical {
			var digest []byte
			if _, err := asn1.Unmarshal(ext.Value, &digest); err == nil && bytes.Equal(digest, want[:]) {
				return nil
			}
		}
	}
	return fmt.Errorf("incorrect ACME validation extension")
}

func mockTLSPlan(t *testing.T, ca *mockACME, cacheDir string) *serverTLS {
	t.Helper()
	p, err := newServerTLS(tlsSettings{auto: true, domains: []string{"example.com"}, cacheDir: cacheDir, directoryURL: ca.server.URL + "/directory"})
	if err != nil {
		t.Fatal(err)
	}
	p.manager.Client.HTTPClient = ca.server.Client()
	return p
}

func dialTestTLS(addr, sni string, roots *x509.CertPool) (*tls.Conn, error) {
	return tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", addr, &tls.Config{ServerName: sni, RootCAs: roots, NextProtos: []string{"http/1.1"}, ClientSessionCache: nil})
}

func assertHTTPConnection(t *testing.T, c *tls.Conn) {
	t.Helper()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c, "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	r, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil || string(body) != "ok" {
		t.Fatalf("HTTP response %q: %v", body, err)
	}
	c.SetDeadline(time.Time{})
}

func TestAutoTLSACMEIssuanceConcurrentRestartAndSNI(t *testing.T) {
	t.Parallel()
	ca := newMockACME(t, 0)
	cache := t.TempDir()
	p := mockTLSPlan(t, ca, cache)
	srv, addr := startTLSListener(t, p)
	ca.mu.Lock()
	ca.addr = addr
	ca.mu.Unlock()
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := dialTestTLS(addr, "example.com", ca.roots)
			if err == nil {
				c.Close()
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if ca.orderAttempts.Load() != 1 || ca.challenges.Load() != 1 {
		t.Fatal("concurrent issuance was not coordinated")
	}
	c, err := dialTestTLS(addr, "example.com", ca.roots)
	if err != nil {
		t.Fatal(err)
	}
	serial := c.ConnectionState().PeerCertificates[0].SerialNumber.String()
	assertHTTPConnection(t, c)
	c.Close()
	// HTTP/2 is verified over an actual HTTPS request, not just the ALPN list.
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: ca.roots}, ForceAttemptHTTP2: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}}
	defer transport.CloseIdleConnections()
	h2, err := (&http.Client{Transport: transport, Timeout: 5 * time.Second}).Get("https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	h2.Body.Close()
	if h2.ProtoMajor != 2 {
		t.Fatal("HTTP/2 unavailable:", h2.Proto)
	}
	// Add a disallowed name to cache, including a token, to test the wrapper boundary.
	data, err := p.manager.Cache.Get(context.Background(), "example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"blocked.example.com", "blocked.example.com+token"} {
		if err := p.manager.Cache.Put(context.Background(), key, data); err != nil {
			t.Fatal(err)
		}
	}
	for _, sni := range []string{"blocked.example.com", ""} {
		cfg := &tls.Config{ServerName: sni, InsecureSkipVerify: true, NextProtos: []string{acme.ALPNProto}}
		if c, err := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", addr, cfg); err == nil {
			c.Close()
			t.Fatal("accepted disallowed SNI", sni)
		}
	}
	if ca.orderAttempts.Load() != 1 {
		t.Fatal("disallowed SNI issued a certificate")
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	addr2, stop := startTLSProcess(t, p)
	defer stop()
	c2, err := dialTestTLS(addr2, "example.com", ca.roots)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if c2.ConnectionState().PeerCertificates[0].SerialNumber.String() != serial || ca.orderAttempts.Load() != 1 {
		t.Fatal("restart did not reuse certificate cache")
	}
	assertHTTPConnection(t, c2)
}

// This test binary is also a child server for the real process restart check.
func TestAutoTLSProcessHelper(t *testing.T) {
	if os.Getenv("_SHERRY_TLS_PROCESS_HELPER") != "1" {
		return
	}
	c, err := tlsSettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	p, err := newServerTLS(c)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }), ErrorLog: log.New(io.Discard, "", 0)}
	if err := p.configure(srv, nil); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.serve(srv, ln) }()
	fmt.Println(ln.Addr().String())
	io.Copy(io.Discard, os.Stdin) // Parent closes stdin to request graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != http.ErrServerClosed {
		t.Fatal(err)
	}
}

func startTLSProcess(t *testing.T, p *serverTLS) (string, func()) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAutoTLSProcessHelper$")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if !strings.HasPrefix(upper, "TLSAUTO") && upper != "SSLKEYFILE" && upper != "SSLCERTIFICATION" && upper != "_SHERRY_TLS_PROCESS_HELPER" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "_SHERRY_TLS_PROCESS_HELPER=1", "TLSAutoEnabled=true", "TLSAutoDomains=example.com", "TLSAutoAcceptTOS=true", "TLSAutoCacheDir="+p.settings.cacheDir, "TLSAutoDirectoryURL="+p.settings.directoryURL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var stderr lockedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			stdin.Close()
			if err := cmd.Wait(); err != nil {
				t.Errorf("TLS child: %v: %s", err, stderr.String())
			}
			cancel()
		})
	}
	t.Cleanup(stop)
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("child listener: %v: %s", err, stderr.String())
	}
	addr := strings.TrimSpace(line)
	if _, _, err := net.SplitHostPort(addr); err != nil {
		t.Fatalf("child address %q", addr)
	}
	return addr, stop
}

func TestAutoTLSACMERenewal(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%t", fail), func(t *testing.T) {
			t.Parallel()
			// RenewBefore triggers an immediate background renewal while the first
			// certificate is still valid, without changing autocert internals.
			ca := newMockACME(t, 20*time.Second)
			p := mockTLSPlan(t, ca, t.TempDir())
			p.manager.RenewBefore = time.Minute
			_, addr := startTLSListener(t, p)
			ca.mu.Lock()
			ca.addr = addr
			ca.mu.Unlock()
			// Set failure only after first issuance, inside the CA's finalize response.
			// A gate transport below makes this deterministic before renewal starts.
			base := p.manager.Client.HTTPClient.Transport
			p.manager.Client.HTTPClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				resp, err := base.RoundTrip(r)
				if fail && strings.HasPrefix(r.URL.Path, "/cert/") {
					ca.failOrders.Store(true)
				}
				return resp, err
			})
			old, err := dialTestTLS(addr, "example.com", ca.roots)
			if err != nil {
				t.Fatal(err)
			}
			defer old.Close()
			assertHTTPConnection(t, old)
			original := old.ConnectionState().PeerCertificates[0]
			deadline := time.Now().Add(8 * time.Second)
			var fresh *tls.Conn
			for time.Now().Before(deadline) {
				if ca.orderAttempts.Load() >= 2 {
					fresh, err = dialTestTLS(addr, "example.com", ca.roots)
					if err != nil {
						t.Fatal(err)
					}
					same := fresh.ConnectionState().PeerCertificates[0].SerialNumber.Cmp(original.SerialNumber) == 0
					if fail || !same {
						break
					}
					fresh.Close()
					fresh = nil
				}
				time.Sleep(20 * time.Millisecond)
			}
			if fresh == nil {
				t.Fatal("renewal was not observed")
			}
			defer fresh.Close()
			state := fresh.ConnectionState()
			if state.DidResume {
				t.Fatal("renewal check resumed a session")
			}
			same := state.PeerCertificates[0].SerialNumber.Cmp(original.SerialNumber) == 0
			if same != fail {
				t.Fatalf("renewal failure=%t, same certificate=%t", fail, same)
			}
			assertHTTPConnection(t, fresh)
			assertHTTPConnection(t, old) // original connection survives renewal/failure
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

func TestAutoTLSACMEInitialFailureRetries(t *testing.T) {
	t.Parallel()
	ca := newMockACME(t, 0)
	ca.failOrders.Store(true)
	p := mockTLSPlan(t, ca, t.TempDir())
	core, observed := observer.New(zap.WarnLevel)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") })}
	if err := p.configure(srv, zap.New(core)); err != nil {
		t.Fatal(err)
	}
	var standardLog lockedBuffer
	srv.ErrorLog = log.New(&standardLog, "", 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ca.mu.Lock()
	ca.addr = addr
	ca.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- p.serve(srv, ln) }()
	t.Cleanup(func() { srv.Close(); <-done })
	if c, err := dialTestTLS(addr, "example.com", ca.roots); err == nil {
		c.Close()
		t.Fatal("initial issuance should fail")
	}
	ca.failOrders.Store(false)
	// autocert v0.55.0 keeps failed issuance state for one minute. Exercise its
	// real backoff rather than modifying unexported timers or bypassing the manager.
	deadline := time.Now().Add(65 * time.Second)
	var recovered *tls.Conn
	for time.Now().Before(deadline) {
		c, err := dialTestTLS(addr, "example.com", ca.roots)
		if err == nil {
			recovered = c
			break
		}
		time.Sleep(time.Second)
	}
	if recovered == nil {
		t.Fatal("issuance did not recover after backoff")
	}
	defer recovered.Close()
	assertHTTPConnection(t, recovered)
	if ca.orderAttempts.Load() != 2 {
		t.Fatal("backoff did not limit retries", ca.orderAttempts.Load())
	}
	// Close the listener; the synchronized log buffer also covers handshake logging.
	recovered.Close()
	srv.Close()
	if strings.Contains(standardLog.String(), "REMOTE_SECRET_MARKER") {
		t.Fatal("remote error body leaked into TLS logs")
	}
	if observed.FilterMessage("Auto TLS certificate unavailable").Len() == 0 {
		t.Fatal("missing certificate failure log")
	}
}

func TestAutoTLSGracefulShutdown(t *testing.T) {
	t.Parallel()
	ca := newMockACME(t, 0)
	p := mockTLSPlan(t, ca, t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	srv, addr := startTLSListener(t, p, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		io.WriteString(w, "finished")
	}))
	t.Cleanup(unblock)
	ca.mu.Lock()
	ca.addr = addr
	ca.mu.Unlock()
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: ca.roots},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	defer transport.CloseIdleConnections()
	response := make(chan error, 1)
	go func() {
		r, err := (&http.Client{Transport: transport, Timeout: 5 * time.Second}).Get("https://example.com/")
		if err == nil {
			data, readErr := io.ReadAll(r.Body)
			r.Body.Close()
			if readErr != nil {
				err = readErr
			} else if string(data) != "finished" {
				err = fmt.Errorf("incomplete response %q", data)
			}
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not enter handler")
	}
	shutdown := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		shutdown <- srv.Shutdown(ctx)
	}()
	select {
	case err := <-shutdown:
		t.Fatalf("shutdown returned before active request finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	unblock()
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-shutdown; err != nil {
		t.Fatal(err)
	}
}

func TestLegacyModesServe(t *testing.T) {
	ca := newMockACME(t, 0)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(42), DNSNames: []string{"example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca.root, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	for file, data := range map[string][]byte{certFile: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyFile: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})} {
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []tlsSettings{{}, {certFile: certFile, keyFile: keyFile}} {
		p, err := newServerTLS(c)
		if err != nil {
			t.Fatal(err)
		}
		_, addr := startTLSListener(t, p)
		if p.mode() == "http" {
			r, err := http.Get("http://" + addr)
			if err != nil {
				t.Fatal(err)
			}
			r.Body.Close()
			if r.StatusCode != 200 {
				t.Fatal(r.Status)
			}
		} else {
			conn, err := dialTestTLS(addr, "example.com", ca.roots)
			if err != nil {
				t.Fatal(err)
			}
			assertHTTPConnection(t, conn)
			conn.Close()
		}
		if ca.orderAttempts.Load() != 0 {
			t.Fatal("legacy mode used ACME")
		}
	}
}
