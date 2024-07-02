package gredis

import (
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
)

func TestGRedis(t *testing.T) {
	var mu sync.RWMutex
	var items = make(map[string][]byte, 1024)
	gr := NewGRedis(func(cmd resp.Command) (out []byte, err error) {
		switch strings.ToLower(string(cmd.Args[0])) {
		default:
			out = resp.AppendError(out, "ERR unknown command '"+string(cmd.Args[0])+"'")
		case "ping":
			out = resp.AppendString(out, "PONG")
		case "quit":
			out = resp.AppendString(out, "OK")
			err = io.EOF
		case "set":
			if len(cmd.Args) != 3 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+string(cmd.Args[0])+"' command")
				break
			}
			mu.Lock()
			items[string(cmd.Args[1])] = cmd.Args[2]
			mu.Unlock()
			out = resp.AppendString(out, "OK")
		case "get":
			if len(cmd.Args) != 2 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+string(cmd.Args[0])+"' command")
				break
			}
			mu.RLock()
			val, ok := items[string(cmd.Args[1])]
			mu.RUnlock()
			if !ok {
				out = resp.AppendNull(out)
			} else {
				out = resp.AppendBulk(out, val)
			}
		case "del":
			if len(cmd.Args) != 2 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+string(cmd.Args[0])+"' command")
				break
			}
			mu.Lock()
			_, ok := items[string(cmd.Args[1])]
			delete(items, string(cmd.Args[1]))
			mu.Unlock()
			if !ok {
				out = resp.AppendInt(out, 0)
			} else {
				out = resp.AppendInt(out, 1)
			}
		case "config":
			// This simple (blank) response is only here to allow for the
			// redis-benchmark command to work with this example.
			out = resp.AppendArray(out, 2)
			out = resp.AppendBulk(out, cmd.Args[2])
			out = resp.AppendBulkString(out, "")
		}
		return
	})

	err := gr.Serve("tcp://:6380", gnet.WithMulticore(true), gnet.WithReuseAddr(true))
	if err != nil {
		t.Fatal(err)
	}
}
