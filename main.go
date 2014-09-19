package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

const (
	ExitCodeError = 111
	NoIndex       = int64(-1)
)

var Version string

var TrapSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT}

type Options struct {
	Wait     bool
	ExitCode int
}

type KVResult struct {
	CreateIndex int64
	ModifyIndex int64
	LockIndex   int64
	Key         string
	Flags       int64
	Value       string
	Session     string
}

type KVResults []KVResult

type Session struct {
	ID string
}

func main() {
	code := run()
	os.Exit(code)
}

func parseOptions() (opt *Options, key string, program string, args []string) {
	var showVersion bool
	var noDelay bool
	var delay bool
	var exitZero bool
	var exitNonZero bool

	flag.Usage = usage
	flag.BoolVar(&noDelay, "n", false, "No delay. If KEY is locked by another process, go-redis-setlock gives up.")
	flag.BoolVar(&delay, "N", true, "(Default.) Delay. If KEY is locked by another process, go-redis-setlock waits until it can obtain a new lock.")
	flag.BoolVar(&exitZero, "x", false, "If KEY is locked, go-redis-setlock exits zero.")
	flag.BoolVar(&exitNonZero, "X", true, "(Default.) If KEY is locked, go-redis-setlock prints an error message and exits nonzero.")
	flag.BoolVar(&showVersion, "version", false, fmt.Sprintf("version %s", Version))
	flag.Parse()

	if showVersion {
		fmt.Fprintf(os.Stderr, "version: %s\n", Version)
		os.Exit(0)
	}

	opt = &Options{
		Wait:     true,
		ExitCode: ExitCodeError,
	}
	if noDelay {
		opt.Wait = false
	}
	if exitZero {
		opt.ExitCode = 0
	}

	remainArgs := flag.Args()
	if len(remainArgs) >= 2 {
		key = remainArgs[0]
		program = remainArgs[1]
		if len(remainArgs) >= 3 {
			args = remainArgs[2:]
		}
	} else {
		usage()
	}

	return opt, key, program, args
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage:\n    consul-lock [-nNxX] KEY program [ arg ... ]\n\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func run() int {
	opt, key, program, args := parseOptions()
	client := &http.Client{}
	sessionID, err := tryGetLock(client, opt, key)
	if err == nil {
		defer releaseLock(client, opt, key, sessionID)
		code := invokeCommand(program, args)
		return code
	} else {
		log.Println(err)
		return opt.ExitCode
	}
}

func tryGetLock(client *http.Client, opt *Options, key string) (sessionID string, err error) {
	var index int64
	for {
		url := "http://localhost:8500/v1/kv/locks/" + key
		if index > 0 {
			url = url + fmt.Sprintf("?wait=10s&index=%d", index)
		}
		req, _ := http.NewRequest("GET", url, nil)
		var kvrs KVResults
		res, newIndex, err := callAPI(client, req, &kvrs)
		if err != nil {
			return "", err
		}
		if newIndex != NoIndex {
			index = newIndex
			log.Println("new index", index)
		}
		try := false
		if res.StatusCode == http.StatusOK {
			if len(kvrs) == 0 {
				return "", fmt.Errorf("invalid response /v1/kv/%s", key)
			}
			log.Printf("%#v", kvrs)
			kvr := kvrs[0]
			if kvr.Session == "" {
				try = true
			}
		} else if res.StatusCode == http.StatusNotFound {
			try = true
		}
		if !try {
			continue
		}
		// not locked. try get lock
		req, _ = http.NewRequest("PUT", "http://localhost:8500/v1/session/create", nil)
		var session Session
		res, _, err = callAPI(client, req, &session)
		if err != nil {
			return "", err
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("invalid status")
		}
		log.Println("sessionID", session.ID)
		body := strings.NewReader(key)
		req, _ = http.NewRequest("PUT", "http://localhost:8500/v1/kv/locks/"+key+"?acquire="+session.ID, body)
		var ok bool
		res, _, err = callAPI(client, req, &ok)
		if err != nil {
			return "", err
		}
		if res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("invalid status")
		}
		if ok {
			return session.ID, nil
		} else if !opt.Wait {
			return "", fmt.Errorf("unable to lock")
		}
	}
}

func callAPI(client *http.Client, req *http.Request, result interface{}) (*http.Response, int64, error) {
	log.Println("callAPI", req.Method, req.URL)
	res, err := client.Do(req)
	if err != nil {
		return res, NoIndex, err
	}
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(result)
	if err != nil && err != io.EOF {
		return res, NoIndex, err
	}
	consulIndex := res.Header.Get("X-Consul-Index")
	if consulIndex != "" {
		newIndex, _ := strconv.ParseInt(consulIndex, 10, 64)
		return res, newIndex, nil
	}
	return res, NoIndex, nil
}

func releaseLock(client *http.Client, opt *Options, key string, sessionID string) error {
	reqDestroySession, _ := http.NewRequest("PUT", "http://localhost:8500/v1/session/destroy/"+key, nil)
	reqDeleteKV, _ := http.NewRequest("DELETE", "http://localhost:8500/v1/kv/locks/"+key, nil)
	reqs := []*http.Request{reqDestroySession, reqDeleteKV}
	for _, req := range reqs {
		var ok bool
		res, _, err := callAPI(client, req, &ok)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("invalid status")
		}
	}
	return nil
}

func invokeCommand(program string, args []string) (code int) {
	cmd := exec.Command(program, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Println(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Println(err)
	}
	go func() {
		_, err := io.Copy(stdin, os.Stdin)
		if err == nil {
			stdin.Close()
		} else {
			log.Println(err)
			stdin.Close()
		}
	}()
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	var cmdErr error
	cmdCh := make(chan error)
	go func() {
		cmdCh <- cmd.Wait()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, TrapSignals...)
	select {
	case s := <-signalCh:
		cmd.Process.Signal(s) // forward to child
		switch sig := s.(type) {
		case syscall.Signal:
			code = int(sig)
			log.Printf("Got signal: %s(%d)", sig, sig)
		default:
			code = -1
		}
		<-cmdCh
	case cmdErr = <-cmdCh:
	}

	// http://qiita.com/hnakamur/items/5e6f22bda8334e190f63
	if cmdErr != nil {
		if e2, ok := cmdErr.(*exec.ExitError); ok {
			if s, ok := e2.Sys().(syscall.WaitStatus); ok {
				code = s.ExitStatus()
			} else {
				log.Println("Unimplemented for system where exec.ExitError.Sys() is not syscall.WaitStatus.")
				return ExitCodeError
			}
		}
	}
	return code
}
