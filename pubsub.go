package gredis

import (
	"regexp"
	"sync"

	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
)

type subChannel struct {
	conn     gnet.Conn
	channels []string
	pattern  bool
}

func newPubSub() *pubSub {
	return &pubSub{
		conns: make(map[gnet.Conn]*subChannel),
		psubs: make(map[string][]gnet.Conn),
		subs:  make(map[string][]gnet.Conn),
	}
}

type pubSub struct {
	rw    sync.RWMutex
	conns map[gnet.Conn]*subChannel
	psubs map[string][]gnet.Conn
	subs  map[string][]gnet.Conn
}

func (p *pubSub) Subscribe(conn gnet.Conn, pattern bool, channels []string) {
	p.rw.Lock()
	defer p.rw.Unlock()
	sc := &subChannel{channels: channels, pattern: pattern, conn: conn}
	p.conns[conn] = sc
	for _, channel := range channels {
		if pattern {
			p.psubs[channel] = append(p.psubs[channel], conn)
		} else {
			p.subs[channel] = append(p.subs[channel], conn)
		}
	}

	// send a message to the client
	var outs [][]byte
	for i, channel := range channels {
		var out []byte
		out = resp.AppendArray(out, 3)
		if pattern {
			out = resp.AppendBulkString(out, "psubscribe")
		} else {
			out = resp.AppendBulkString(out, "subscribe")
		}
		out = resp.AppendBulkString(out, channel)
		out = resp.AppendInt(out, int64(i+1))
		outs = append(outs, out)
	}
	if len(outs) > 0 {
		_, _ = conn.Writev(outs)
	}
}

func (p *pubSub) Publish(channel, message string) int {
	p.rw.RLock()
	defer p.rw.RUnlock()
	var sent int
	if conns, ok := p.subs[channel]; ok {
		for _, conn := range conns {
			_, _ = conn.Write(p.writeMessage(false, "", channel, message))
			sent++
		}
	}

	for pchan, conns := range p.psubs {
		re, err := regexp.Compile(pchan)
		if err != nil {
			continue
		}
		if re.MatchString(channel) {
			for _, conn := range conns {
				_, _ = conn.Write(p.writeMessage(true, pchan, channel, message))
				sent++
			}
		}
	}

	return sent
}

func (p *pubSub) writeMessage(pat bool, pchan, channel, msg string) []byte {
	var out []byte
	if pat {
		out = resp.AppendArray(out, 4)
		out = resp.AppendBulkString(out, "pmessage")
		out = resp.AppendBulkString(out, pchan)
		out = resp.AppendBulkString(out, channel)
		out = resp.AppendBulkString(out, msg)
	} else {
		out = resp.AppendArray(out, 3)
		out = resp.AppendBulkString(out, "message")
		out = resp.AppendBulkString(out, channel)
		out = resp.AppendBulkString(out, msg)
	}
	return out
}

func (p *pubSub) OnClose(conn gnet.Conn) {
	p.rw.Lock()
	defer p.rw.Unlock()
	if sc, ok := p.conns[conn]; ok {
		for _, channel := range sc.channels {
			if sc.pattern {
				delete(p.psubs, channel)
			} else {
				delete(p.subs, channel)
			}
		}
		delete(p.conns, conn)
	}
}
