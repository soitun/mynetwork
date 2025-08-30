//go:build windows
// +build windows

package dns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/miekg/dns"
	"github.com/multiformats/go-multibase"
	"github.com/soitun/mynetwork/config"
)

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func domainSuffix(config config.Config) string {
	if config.Interface == "mynetwork" {
		return "mynetwork."
	} else {
		return fmt.Sprintf("%s.mynetwork.", config.Interface)
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func withDomainSuffix(config config.Config, str string) string {
	return fmt.Sprintf("%s.%s", str, domainSuffix(config))
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func mkAliasRecord(config config.Config, alias string, serviceName string, p peer.ID) *dns.CNAME {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	var aliasWithSvc string
	var cidWithSvc string
	if serviceName == "" {
		aliasWithSvc = alias
		cidWithSvc = cid
	} else {
		aliasWithSvc = serviceName + "." + alias
		cidWithSvc = serviceName + "." + cid
	}

	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(config, aliasWithSvc),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Target: withDomainSuffix(config, cidWithSvc),
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func mkIDRecord4(config config.Config, p peer.ID, addr net.IP) *dns.A {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(config, cid),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: addr,
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func mkIDRecord6(cfg config.Config, p peer.ID, serviceName string, addr net.IP) *dns.AAAA {
	cid, _ := peer.ToCid(p).StringOfBase(multibase.Base36)
	var cidWithSvc string
	if serviceName == "" {
		cidWithSvc = cid
	} else {
		cidWithSvc = serviceName + "." + cid
	}

	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   withDomainSuffix(cfg, cidWithSvc),
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		AAAA: addr,
	}
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func writeResponse(msg *dns.Msg, q dns.Question, p peer.ID, addr net.IP) {
	// Implementation for Windows - simplified version
}

// -----------------------------------------------------------------------------------------------------------------------------------------------------
func MagicDnsServer(ctx context.Context, config config.Config, node host.Host) {
	dns.HandleFunc(domainSuffix(config), func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)

		for _, q := range r.Question {
			switch q.Qtype {
			case dns.TypeA:
				fallthrough
			case dns.TypeAAAA:
				nameParts := strings.Split(strings.TrimSuffix(q.Name, "."+domainSuffix(config)), ".")
				var qNodeName string
				var qServiceName string
				if len(nameParts) == 2 {
					qServiceName = nameParts[0]
					qNodeName = nameParts[1]
				} else if len(nameParts) == 1 {
					qNodeName = nameParts[0]
				} else {
					return
				}
				isService := qServiceName != ""
				if qpeer, err := peer.Decode(qNodeName); err == nil {
					if qpeer == node.ID() {
						if !isService {
							m.Answer = append(m.Answer, mkIDRecord4(config, node.ID(), config.BuiltinAddr4))
						}
						m.Answer = append(m.Answer, mkIDRecord6(config, node.ID(), qServiceName, config.BuiltinAddr6))
					} else {
						for _, p := range config.Peers {
							if p.ID == qpeer {
								if !isService {
									m.Answer = append(m.Answer, mkIDRecord4(config, p.ID, p.BuiltinAddr4))
								}
								m.Answer = append(m.Answer, mkIDRecord6(config, p.ID, qServiceName, p.BuiltinAddr6))
								break
							}
						}
					}
				} else {
					// Handle peer lookup by name
					if qpeer, ok := config.PeerLookup.ByName[qNodeName]; ok {
						if !isService {
							m.Answer = append(m.Answer, mkIDRecord4(config, qpeer.ID, qpeer.BuiltinAddr4))
						}
						m.Answer = append(m.Answer, mkIDRecord6(config, qpeer.ID, qServiceName, qpeer.BuiltinAddr6))
					}
				}
			case dns.TypeCNAME:
				// Handle CNAME records
			}
		}

		w.WriteMsg(m)
	})

	// Start DNS servers on different protocols
	dnsServerAddr := "127.0.0.1"
	dnsServerPort := uint16(5333)

	for _, protocol := range []string{"tcp", "udp"} {
		sv := &dns.Server{
			Addr: fmt.Sprintf("%s:%d", dnsServerAddr, dnsServerPort),
			Net:  protocol,
		}
		fmt.Printf("[-] Starting DNS server on /ip4/%s/%s/%d\n", dnsServerAddr, sv.Net, dnsServerPort)
		go func(server *dns.Server) {
			if err := server.ListenAndServe(); err != nil {
				fmt.Printf("[!] DNS server error: %s, %s\n", server.Net, err.Error())
			}
		}(sv)
	}

	// On Windows, we don't configure systemd-resolved
	// Users need to manually configure DNS settings
	fmt.Printf("[+] DNS server started. Configure your system to use %s:%d as DNS server\n", dnsServerAddr, dnsServerPort)
}
