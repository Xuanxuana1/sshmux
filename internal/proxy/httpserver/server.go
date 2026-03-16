package httpserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
)

// Run starts an HTTP→SOCKS5 proxy server on httpPort, forwarding all traffic
// through the SOCKS5 proxy at 127.0.0.1:socksPort. Blocks until the process
// is killed.
func Run(httpPort, socksPort int) error {
	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", httpPort),
		Handler: &proxyHandler{socksAddr: socksAddr},
	}
	return srv.ListenAndServe()
}

type proxyHandler struct {
	socksAddr string
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		h.handleCONNECT(w, r)
	} else {
		h.handleHTTP(w, r)
	}
}

// handleCONNECT handles HTTPS tunneling via HTTP CONNECT.
func (h *proxyHandler) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	upstream, err := socks5Dial(h.socksAddr, r.Host)
	if err != nil {
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	w.WriteHeader(http.StatusOK)

	hij, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	client, _, err := hij.Hijack()
	if err != nil {
		return
	}
	defer client.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, client); done <- struct{}{} }()
	go func() { io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

// handleHTTP forwards plain HTTP requests through SOCKS5.
func (h *proxyHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Host == "" {
		http.Error(w, "no host in request", http.StatusBadRequest)
		return
	}
	host := r.URL.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":80"
	}

	upstream, err := socks5Dial(h.socksAddr, host)
	if err != nil {
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	r.Header.Del("Proxy-Connection")
	if err := r.Write(upstream); err != nil {
		http.Error(w, "write: "+err.Error(), http.StatusBadGateway)
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(upstream), r)
	if err != nil {
		http.Error(w, "read: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// socks5Dial connects to addr through the SOCKS5 proxy at socksAddr.
func socks5Dial(socksAddr, addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", socksAddr)
	if err != nil {
		return nil, fmt.Errorf("connect socks5 %s: %w", socksAddr, err)
	}

	// No-auth handshake
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 handshake: %w", err)
	}
	var hs [2]byte
	if _, err := io.ReadFull(conn, hs[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 handshake reply: %w", err)
	}
	if hs[0] != 5 || hs[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("socks5 auth rejected (method=%d)", hs[1])
	}

	// CONNECT request
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse addr %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		conn.Close()
		return nil, fmt.Errorf("invalid port %q", portStr)
	}

	req := make([]byte, 0, 7+len(host))
	req = append(req, 5, 1, 0, 3, byte(len(host)))
	req = append(req, []byte(host)...)
	req = append(req, byte(port>>8), byte(port&0xff))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect request: %w", err)
	}

	// Response header (VER REP RSV ATYP)
	var rh [4]byte
	if _, err := io.ReadFull(conn, rh[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect reply: %w", err)
	}
	if rh[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect failed (code=%d)", rh[1])
	}

	// Consume bound address (required by protocol, not used)
	switch rh[3] {
	case 1: // IPv4
		io.ReadFull(conn, make([]byte, 6))
	case 3: // Domain
		var n [1]byte
		io.ReadFull(conn, n[:])
		io.ReadFull(conn, make([]byte, int(n[0])+2))
	case 4: // IPv6
		io.ReadFull(conn, make([]byte, 18))
	}

	return conn, nil
}
