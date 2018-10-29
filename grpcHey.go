package main

import (
	"flag"
	"fmt"
	"github.com/rakyll/grpc-hey/requester"
	"math"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var (

	// 并发数
	c = flag.Int("c", 50, "")
	// 总的请求数
	n = flag.Int("n", 200, "")
	q = flag.Float64("q", 0, "")
	t = flag.Int("t", 20, "")
	z = flag.Duration("z", 0, "")

	cpus = flag.Int("cpus", runtime.GOMAXPROCS(-1), "")
)

var usage = `Usage: hey [options...] <url>

Options:
  -n  Number of requests to run. Default is 200.
  -c  Number of requests to run concurrently. Total number of requests cannot
      be smaller than the concurrency level. Default is 50.
  -q  Rate limit, in queries per second (QPS). Default is no rate limit.
  -z  Duration of application to send requests. When duration is reached,
      application stops and exits. If duration is specified, n is ignored.
      Examples: -z 10s -z 3m.
  -t  Timeout for each request in seconds. Default is 20, use 0 for infinite.
  -cpus                 Number of used cpu cores.
                        (default for current machine is %d cores)
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}

	flag.Parse()
	// if flag.NArg() < 1 {
	// 	usageAndExit("")
	// }

	runtime.GOMAXPROCS(*cpus)
	num := *n
	conc := *c
	q := *q
	dur := *z

	if dur > 0 {
		num = math.MaxInt32
		if conc <= 0 {
			usageAndExit("-c cannot be smaller than 1.")
		}
	} else {
		if num <= 0 || conc <= 0 {
			usageAndExit("-n and -c cannot be smaller than 1.")
		}

		if num < conc {
			usageAndExit("-n cannot be less than -c.")
		}
	}

	w := &requester.RpcWork{
		N:                  num,
		C:                  conc,
		QPS:                q,
		Timeout:            *t,
	}
	w.Init()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		w.Stop()
	}()
	if dur > 0 {
		go func() {
			time.Sleep(dur)
			w.Stop()
		}()
	}
	w.Run()
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}