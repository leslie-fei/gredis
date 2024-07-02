package main

import (
	"flag"
	"io"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"sync"
	"unsafe"

	"github.com/leslie-fei/gredis"
	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
)

func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	var mu sync.RWMutex
	var items = make(map[string][]byte, 1024)
	var network string
	var addr string
	var multicore bool
	var reusePort bool
	flag.StringVar(&network, "network", "tcp", "server network (default \"tcp\")")
	flag.StringVar(&addr, "addr", ":6380", "server addr (default \":6380\")")
	flag.BoolVar(&multicore, "multicore", true, "multicore")
	flag.BoolVar(&reusePort, "reusePort", false, "reusePort")
	flag.Parse()

	gr := gredis.NewGRedis(func(cmd resp.Command) (out []byte, err error) {
		switch strings.ToLower(b2s(cmd.Args[0])) {
		default:
			out = resp.AppendError(out, "ERR unknown command '"+b2s(cmd.Args[0])+"'")
		case "ping":
			out = resp.AppendString(out, "PONG")
		case "quit":
			out = resp.AppendString(out, "OK")
			err = io.EOF
		case "set":
			if len(cmd.Args) != 3 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+b2s(cmd.Args[0])+"' command")
				break
			}
			mu.Lock()
			items[b2s(cmd.Args[1])] = cmd.Args[2]
			mu.Unlock()
			out = resp.AppendString(out, "OK")
		case "get":
			if len(cmd.Args) != 2 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+b2s(cmd.Args[0])+"' command")
				break
			}
			mu.RLock()
			val, ok := items[b2s(cmd.Args[1])]
			mu.RUnlock()
			if !ok {
				out = resp.AppendNull(out)
			} else {
				out = resp.AppendBulk(out, val)
			}
		case "del":
			if len(cmd.Args) != 2 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+b2s(cmd.Args[0])+"' command")
				break
			}
			mu.Lock()
			_, ok := items[b2s(cmd.Args[1])]
			delete(items, b2s(cmd.Args[1]))
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

	err := gr.Serve("tcp://:6380", gnet.WithMulticore(multicore), gnet.WithReuseAddr(reusePort))
	if err != nil {
		panic(err)
	}
}

func s2b(s string) []byte {
	sp := unsafe.StringData(s)
	return unsafe.Slice(sp, len(s))
}

func b2s(bb []byte) string {
	bp := unsafe.SliceData(bb)
	return unsafe.String(bp, len(bb))
}
