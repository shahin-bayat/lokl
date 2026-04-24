package proxy

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNSServerAnswers(t *testing.T) {
	srv, addr := startTestDNSServer(t, []string{"sellify.shop"})
	t.Cleanup(func() { _ = srv.Shutdown() })

	tests := []struct {
		name string
		q    string
		qt   uint16
		rc   int
		ip   string
	}{
		{"A in scope", "x.sellify.shop.", dns.TypeA, dns.RcodeSuccess, "127.0.0.1"},
		{"AAAA in scope", "x.sellify.shop.", dns.TypeAAAA, dns.RcodeSuccess, "::1"},
		{"out of scope", "other.test.", dns.TypeA, dns.RcodeRefused, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(tc.q, tc.qt)
			c := &dns.Client{Net: "udp", Timeout: 500 * time.Millisecond}
			resp, _, err := c.Exchange(m, addr)
			if err != nil {
				t.Fatalf("exchange: %v", err)
			}
			if resp.Rcode != tc.rc {
				t.Fatalf("rcode=%d want %d", resp.Rcode, tc.rc)
			}
			if tc.ip == "" {
				return
			}
			if len(resp.Answer) == 0 {
				t.Fatal("no answer records")
			}
			switch rr := resp.Answer[0].(type) {
			case *dns.A:
				if rr.A.String() != tc.ip {
					t.Fatalf("A=%s want %s", rr.A, tc.ip)
				}
			case *dns.AAAA:
				if rr.AAAA.String() != tc.ip {
					t.Fatalf("AAAA=%s want %s", rr.AAAA, tc.ip)
				}
			default:
				t.Fatalf("unexpected answer type %T", rr)
			}
		})
	}
}

func startTestDNSServer(t *testing.T, parents []string) (*dnsServer, string) {
	t.Helper()
	srv, err := newDNSServer("127.0.0.1:0", parents)
	if err != nil {
		t.Fatalf("new dns server: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	for i := 0; i < 50 && srv.LocalAddr() == nil; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.LocalAddr() == nil {
		t.Fatal("server did not bind in time")
	}
	return srv, srv.LocalAddr().String()
}
