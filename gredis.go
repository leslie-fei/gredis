package gredis

import (
	"bytes"
	"errors"
	"io"
	"sync"

	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type CommandHandler func(conn gnet.Conn, cmd resp.Command) (out []byte, err error)

type GRedis interface {
	Serve(addr string, options ...gnet.Option) error
	OnCommand(h CommandHandler)
	Subscribe(conn gnet.Conn, pattern bool, channels []string)
	Publish(channel, message string) int
}

func NewGRedis() GRedis {
	return &gRedis{buffers: make(map[gnet.Conn]*connBuffer, 1024), pubSub: newPubSub()}
}

type connBuffer struct {
	buf     bytes.Buffer
	command []resp.Command
}

type gRedis struct {
	gnet.BuiltinEventEngine
	handler CommandHandler
	rw      sync.RWMutex
	buffers map[gnet.Conn]*connBuffer
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
	gr.buffers[c] = new(connBuffer)
	return
}

func (gr *gRedis) OnClose(c gnet.Conn, _ error) (action gnet.Action) {
	gr.rw.Lock()
	defer gr.rw.Unlock()
	gr.pubSub.OnClose(c)
	delete(gr.buffers, c)
	return
}

func (gr *gRedis) OnTraffic(c gnet.Conn) (action gnet.Action) {
	gr.rw.RLock()
	defer gr.rw.RUnlock()

	buffer := gr.buffers[c]
	_, err := c.WriteTo(&buffer.buf)
	if err != nil {
		logging.Errorf("OnTraffic writeTo buffer error: %v", err)
		return gnet.Close
	}

	var outs [][]byte
	cmds, lastbyte, err := resp.ReadCommands(buffer.buf.Bytes())
	if err != nil {
		_, _ = c.Write(resp.AppendError(nil, "ERR "+err.Error()))
		return
	}

	buffer.command = append(buffer.command, cmds...)
	buffer.buf.Reset()

	if len(lastbyte) == 0 {
		for _, cmd := range buffer.command {
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
		buffer.command = buffer.command[:0]
		_, _ = c.Writev(outs)
	} else {
		buffer.buf.Write(lastbyte)
	}

	return
}

func (gr *gRedis) Serve(addr string, options ...gnet.Option) error {
	return gnet.Run(gr, addr, options...)
}
