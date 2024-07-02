# Instruction 

high-performance, non-blocking network IO Redis frameworks for Go, based on epoll gnet network model

# Installing

```
go get -u github.com/leslie-fei/gredis
```

# Example

Here's a full example of a Redis clone that accepts:

- SET key value
- GET key
- DEL key
- PING
- QUIT

# Benchmarks

### Redis: Single-threaded, no disk persistence.

```
$ redis-server --port 6379 --appendonly no
```

```
[root@localhost ~]# redis-benchmark -h 127.0.0.1 -p 6379 -t set,get -n 10000000 -q -P 512 -c 512
SET: 987166.81 requests per second
GET: 1220405.12 requests per second
```

### GRedis

```
GOMAXPROCS=1 go run example/main.go
```

```
[root@localhost ~]# redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 10000000 -q -P 512 -c 512
SET: 1199328.38 requests per second
GET: 1218769.00 requests per second
```

```
GOMAXPROCS=0 go run example/main.go
```

```
[root@localhost ~]# redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 10000000 -q -P 512 -c 512
SET: 2018978.38 requests per second
GET: 5611672.50 requests per second
```

### Redcon

```
$ GOMAXPROCS=1 go run example/clone.go
```

```
[root@localhost ~]# redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 10000000 -q -P 512 -c 512
SET: 1331557.88 requests per second
GET: 1638538.38 requests per second
```

```
$ GOMAXPROCS=0 go run example/clone.go
```

```
[root@localhost ~]# redis-benchmark -h 127.0.0.1 -p 6380 -t set,get -n 10000000 -q -P 512 -c 512
SET: 1685772.00 requests per second
GET: 5246589.50 requests per second
```