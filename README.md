# go-redis-setlock

Like the setlock command using Consul session/kv.

## Repository

[github.com/fujiwara/consul-lock](https://github.com/fujiwara/consul-lock)

## Build & Install

Install to $GOPATH.

    $ go get github.com/fujiwara/consul-lock

## Usage

    $ consul-lock [-nNxX] KEY program [ arg ... ]

    -n: No delay. If KEY is locked by another process, consul-lock gives up.
    -N: (Default.) Delay. If KEY is locked by another process, consul-lock waits until it can obtain a new lock.
    -x: If KEY is locked, redis-setlock exits zero.
    -X: (Default.) If KEY is locked, consul-lock prints an error message and exits nonzero.

## LICENSE

MIT