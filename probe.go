package monitor

import (
	stdnet "net"
	"sync/atomic"
	"time"
)

const (
	probeInterval  = 30 * time.Second
	probeTarget    = "223.5.5.5:53" // AliDNS anycast: always-on, no auth, no payload
	probeTimeout   = 3 * time.Second
	latencyUnknown = -1
)

// netProbe tracks outbound internet reachability with a periodic TCP
// handshake to a public DNS resolver. It measures the board's own egress
// path, which is invisible from the LAN side otherwise.
type netProbe struct {
	online atomic.Bool
	rttMs  atomic.Int64 // latencyUnknown (-1) when unreachable
}

func (p *netProbe) start() {
	p.probe() // seed the first reading immediately
	go func() {
		ticker := time.NewTicker(probeInterval)
		defer ticker.Stop()
		for range ticker.C {
			p.probe()
		}
	}()
}

func (p *netProbe) probe() {
	start := time.Now()
	conn, err := stdnet.DialTimeout("tcp", probeTarget, probeTimeout)
	if err != nil {
		p.online.Store(false)
		p.rttMs.Store(latencyUnknown)
		return
	}
	conn.Close()
	p.online.Store(true)
	p.rttMs.Store(time.Since(start).Milliseconds())
}
