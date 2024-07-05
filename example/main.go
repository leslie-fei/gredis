package main

import (
	"flag"
	"io"
	"strings"
	"sync"
	"unsafe"

	"github.com/leslie-fei/gnettls/tls"
	"github.com/leslie-fei/gredis"
	"github.com/leslie-fei/gredis/resp"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

func main() {
	var mu sync.RWMutex
	var items = make(map[string][]byte, 1024)
	var addr string
	var multicore bool
	var reusePort bool
	var enableTLS bool
	flag.StringVar(&addr, "addr", "tcp://:6380", `server addr (default "tcp://:6380")`)
	flag.BoolVar(&multicore, "multicore", true, "multicore")
	flag.BoolVar(&reusePort, "reusePort", false, "reusePort")
	flag.BoolVar(&enableTLS, "tls", false, "enable TLS")
	flag.Parse()

	logging.Infof("addr: %s, multicore: %v, reusePort: %v, tls: %v", addr, multicore, reusePort, enableTLS)

	gr := gredis.NewGRedis()
	gr.OnCommand(func(conn gnet.Conn, cmd resp.Command) (out []byte, err error) {
		switch strings.ToLower(b2s(cmd.Args[0])) {
		case "publish":
			// Publish to all pub/sub subscribers and return the number of
			// messages that were sent.
			if len(cmd.Args) != 3 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+string(cmd.Args[0])+"' command")
				return
			}
			count := gr.Publish(string(cmd.Args[1]), string(cmd.Args[2]))
			out = resp.AppendInt(out, int64(count))
		case "subscribe", "psubscribe":
			// Subscribe to a pub/sub channel. The `Psubscribe` and
			// `Subscribe` operations will detach the connection from the
			// event handler and manage all network I/O for this connection
			// in the background.
			if len(cmd.Args) < 2 {
				out = resp.AppendError(out, "ERR wrong number of arguments for '"+string(cmd.Args[0])+"' command")
				return
			}
			command := strings.ToLower(string(cmd.Args[0]))
			channels := make([]string, 0, len(cmd.Args))
			pattern := false
			if command == "psubscribe" {
				pattern = true
			}
			for i := 1; i < len(cmd.Args); i++ {
				channels = append(channels, string(cmd.Args[i]))
			}
			gr.Subscribe(conn, pattern, channels)
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

	var tc *tls.Config
	if enableTLS {
		tc = &tls.Config{
			Certificates:       []tls.Certificate{mustLoadCertificate()},
			InsecureSkipVerify: true,
		}
	}

	err := gr.Serve(addr, tc, gnet.WithMulticore(multicore), gnet.WithReusePort(reusePort))
	if err != nil {
		panic(err)
	}
}

const serverKey = `-----BEGIN EC PARAMETERS-----
BggqhkjOPQMBBw==
-----END EC PARAMETERS-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIHg+g2unjA5BkDtXSN9ShN7kbPlbCcqcYdDu+QeV8XWuoAoGCCqGSM49
AwEHoUQDQgAEcZpodWh3SEs5Hh3rrEiu1LZOYSaNIWO34MgRxvqwz1FMpLxNlx0G
cSqrxhPubawptX5MSr02ft32kfOlYbaF5Q==
-----END EC PRIVATE KEY-----
`

const serverCert = `-----BEGIN CERTIFICATE-----
MIIB+TCCAZ+gAwIBAgIJAL05LKXo6PrrMAoGCCqGSM49BAMCMFkxCzAJBgNVBAYT
AkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRn
aXRzIFB0eSBMdGQxEjAQBgNVBAMMCWxvY2FsaG9zdDAeFw0xNTEyMDgxNDAxMTNa
Fw0yNTEyMDUxNDAxMTNaMFkxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0
YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQxEjAQBgNVBAMM
CWxvY2FsaG9zdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABHGaaHVod0hLOR4d
66xIrtS2TmEmjSFjt+DIEcb6sM9RTKS8TZcdBnEqq8YT7m2sKbV+TEq9Nn7d9pHz
pWG2heWjUDBOMB0GA1UdDgQWBBR0fqrecDJ44D/fiYJiOeBzfoqEijAfBgNVHSME
GDAWgBR0fqrecDJ44D/fiYJiOeBzfoqEijAMBgNVHRMEBTADAQH/MAoGCCqGSM49
BAMCA0gAMEUCIEKzVMF3JqjQjuM2rX7Rx8hancI5KJhwfeKu1xbyR7XaAiEA2UT7
1xOP035EcraRmWPe7tO0LpXgMxlh2VItpc2uc2w=
-----END CERTIFICATE-----
`

func mustLoadCertificate() tls.Certificate {
	cer, err := tls.X509KeyPair([]byte(serverCert), []byte(serverKey))
	if err != nil {
		panic(err)
	}
	return cer
}

func s2b(s string) []byte {
	sp := unsafe.StringData(s)
	return unsafe.Slice(sp, len(s))
}

func b2s(bb []byte) string {
	bp := unsafe.SliceData(bb)
	return unsafe.String(bp, len(bb))
}
