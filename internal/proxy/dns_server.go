package proxy

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

const (
	defaultDNSPort = 5454
	dnsTTL         = 5
)

type dnsServer struct {
	addr    string
	parents []string
	conn    *net.UDPConn
	srv     *dns.Server
	mu      sync.Mutex
}

func newDNSServer(addr string, parents []string) (*dnsServer, error) {
	if len(parents) == 0 {
		return nil, fmt.Errorf("dns server requires at least one wildcard parent")
	}
	return &dnsServer{addr: addr, parents: parents}, nil
}

func (s *dnsServer) Start() error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolving dns addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.addr, err)
	}
	s.mu.Lock()
	s.conn = conn
	s.srv = &dns.Server{PacketConn: conn, Handler: dns.HandlerFunc(s.handle)}
	s.mu.Unlock()

	go func() { _ = s.srv.ActivateAndServe() }()
	return nil
}

func (s *dnsServer) Shutdown() error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown()
}

func (s *dnsServer) LocalAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

func (s *dnsServer) handle(w dns.ResponseWriter, req *dns.Msg) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = true

	if len(req.Question) != 1 {
		resp.Rcode = dns.RcodeRefused
		_ = w.WriteMsg(resp)
		return
	}

	q := req.Question[0]
	name := strings.TrimSuffix(strings.ToLower(q.Name), ".")
	if !s.inScope(name) {
		resp.Rcode = dns.RcodeRefused
		_ = w.WriteMsg(resp)
		return
	}

	hdr := dns.RR_Header{Name: q.Name, Class: dns.ClassINET, Ttl: dnsTTL}
	switch q.Qtype {
	case dns.TypeA:
		hdr.Rrtype = dns.TypeA
		resp.Answer = append(resp.Answer, &dns.A{Hdr: hdr, A: net.IPv4(127, 0, 0, 1)})
	case dns.TypeAAAA:
		hdr.Rrtype = dns.TypeAAAA
		resp.Answer = append(resp.Answer, &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP("::1")})
	}
	_ = w.WriteMsg(resp)
}

// inScope reports whether name equals a known parent or is one of its subdomains.
func (s *dnsServer) inScope(name string) bool {
	for _, parent := range s.parents {
		if name == parent {
			return true
		}
		if strings.HasSuffix(name, "."+parent) {
			return true
		}
	}
	return false
}
