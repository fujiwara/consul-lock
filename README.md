# consul-lock

Like the setlock command using Consul session/kv.

## Repository

[github.com/fujiwara/consul-lock](https://github.com/fujiwara/consul-lock)

## Binary releases

[github.com/fujiwara/consul-lock/releases](https://github.com/fujiwara/consul-lock/releases)

## Build & Install

Install to $GOPATH.

    $ go get github.com/fujiwara/consul-lock

## Usage

    $ consul-lock [-nNxX] KEY program [ arg ... ]

    -n: No delay. If KEY is locked by another process, consul-lock gives up.
    -N: (Default.) Delay. If KEY is locked by another process, consul-lock waits until it can obtain a new lock.
    -x: If KEY is locked, consul-lock exits zero.
    -X: (Default.) If KEY is locked, consul-lock prints an error message and exits nonzero.
   -lock-delay=15: Consul session LockDelay seconds

## LICENSE

MIT
