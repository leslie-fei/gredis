package gredis

import (
	"errors"
	"io"
	"sync"

	"github.com/leslie-fei/gnettls"
	"github.com/leslie-fei/gnettls/tls"
	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type CommandHandler func(conn gnet.Conn, cmd resp.Command) (out []byte, err error)

type GRedis interface {
	Serve(addr string, tc *tls.Config, options ...gnet.Option) error
	OnCommand(h CommandHandler)
	Subscribe(conn gnet.Conn, pattern bool, channels []string)
	Publish(channel, message string) int
}

func NewGRedis() GRedis {
	return &gRedis{pubSub: newPubSub()}
}

type connContext struct {
	command []resp.Command
}

type gRedis struct {
	gnet.BuiltinEventEngine
	handler CommandHandler
	rw      sync.RWMutex
	pubSub  *pubSub
}

func (gr *gRedis) OnCommand(h CommandHandler) {
	gr.handler = h
}

func (gr *gRedis) Subscribe(conn gnet.Conn, pattern bool, channels []string) {
	gr.pubSub.Subscribe(conn, pattern, channels)
}

func (gr *gRedis) Publish(channel, message string) int {
	return gr.pubSub.Publish(channel, message)
}

func (gr *gRedis) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	gr.rw.Lock()
	defer gr.rw.Unlock()
	c.SetContext(&connContext{})
	return
}

func (gr *gRedis) OnClose(c gnet.Conn, _ error) (action gnet.Action) {
	gr.rw.Lock()
	defer gr.rw.Unlock()
	gr.pubSub.OnClose(c)
	return
}

func (gr *gRedis) OnTraffic(c gnet.Conn) (action gnet.Action) {
	gr.rw.RLock()
	defer gr.rw.RUnlock()

	ctx := c.Context().(*connContext)
	data, err := c.Peek(c.InboundBuffered())
	if err != nil {
		logging.Errorf("OnTraffic peek error: %v", err)
		return gnet.Close
	}

	cmds, lastbyte, err := resp.ReadCommands(data)
	if err != nil {
		_, _ = c.Write(resp.AppendError(nil, "ERR "+err.Error()))
		return
	}

	if len(cmds) > 0 {
		ctx.command = append(ctx.command, cmds...)
	}
	_, _ = c.Discard(c.InboundBuffered() - len(lastbyte))

	var outs [][]byte
	if len(lastbyte) == 0 && len(ctx.command) > 0 {
		for _, cmd := range ctx.command {
			out, err := gr.handler(c, cmd)
			if errors.Is(err, io.EOF) {
				action = gnet.Close
			}
			if err != nil {
				logging.Errorf("OnTraffic fire command handle error: %v", err)
				action = gnet.Close
			}
			outs = append(outs, out)
		}
		ctx.command = ctx.command[:0]
		_, _ = c.Writev(outs)
	}

	return
}

func (gr *gRedis) Serve(addr string, tc *tls.Config, options ...gnet.Option) error {
	return gnettls.Run(gr, addr, tc, options...)
}
