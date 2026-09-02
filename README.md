# go-reuseport

[![Go Reference](https://pkg.go.dev/badge/github.com/ad3n/go-reuseport.svg)](https://pkg.go.dev/github.com/ad3n/go-reuseport)

This package enables listening and dialing from _the same_ TCP or UDP port.
This means that the following sockopts may be set:

```
SO_REUSEADDR
SO_REUSEPORT
```

This is a simple package to help with address reuse. This is particularly
important when attempting to do TCP NAT holepunching, which requires a process
to both Listen and Dial on the same TCP port. This package provides some
utilities around enabling this behaviour on various OS.

Install it with:

```sh
go get github.com/ad3n/go-reuseport
```

For connection-heavy workloads, create a `Dialer` once and reuse it. This
avoids resolving and allocating the local address for every connection:

```go
d, err := reuseport.NewDialer("tcp", "127.0.0.1:1234", 0)
if err != nil {
	log.Fatal(err)
}
c, err := d.Dial("tcp", "127.0.0.1:1235")
```

## Examples


```Go
// listen on the same port. oh yeah.
l1, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
l2, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
```

```Go
// dial from the same port. oh yeah.
l1, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
l2, _ := reuseport.Listen("tcp", "127.0.0.1:1235")
c, _ := reuseport.Dial("tcp", "127.0.0.1:1234", "127.0.0.1:1235")
```

**Note: cant dial self because tcp/ip stacks use 4-tuples to identify connections, and doing so would clash.**

## Tested

Tested on `darwin`, `linux`, and `windows`.

---

The last gx published version of this module was: 0.2.2: Qme8kdM7thoCqLqd7GYCRqipoZJS64rhJo5MBcTpyWfsL9
