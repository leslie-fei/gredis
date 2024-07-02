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
```