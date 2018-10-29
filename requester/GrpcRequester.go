package requester

import (
	pb "github.com/rakyll/grpc-hey/helloworld"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
)

const maxResult = 1000000

const (
	address = "localhost:50051"
)

type result struct {
	err           error
	statusCode    int
	offset        time.Duration
	duration      time.Duration
	connDuration  time.Duration // connection setup(DNS lookup + Dial up) duration
	dnsDuration   time.Duration // dns lookup duration
	reqDuration   time.Duration // request "write" duration
	resDuration   time.Duration // response "read" duration
	delayDuration time.Duration // delay between response and request
	contentLength int64
}

type RpcWork struct {
	RequestBody pb.HelloRequest

	// 表示总请求数
	N int
	// 并发等级，多少个worker
	C int
	// 超时时间
	Timeout int
	//
	QPS float64

	initOnce sync.Once
	results  chan *result
	stopCh   chan struct{}
	start    time.Duration

	report *report
}

// Init initializes internal data-structures
func (b *RpcWork) Init() {
	b.initOnce.Do(func() {
		b.results = make(chan *result, min(b.C*1000, maxResult))
		b.stopCh = make(chan struct{}, b.C)
	})
}

// Run makes all the requests, prints the summary. It blocks until
// all work is done.
func (b *RpcWork) Run() {
	b.Init()
	b.start = now()
	b.report = newReport(os.Stdout, b.results, "", b.N)
	// Run the reporter first, it polls the result channel until it is closed.
	go func() {
		runReporter(b.report)
	}()
	b.runWorkers()
	b.Finish()
}

func (b *RpcWork) runWorkers() {
	var wg sync.WaitGroup
	wg.Add(b.C)



	// Ignore the case where b.N % b.C != 0.
	for i := 0; i < b.C; i++ {
		go func() {
			b.runWorker(b.N/b.C)
			wg.Done()
		}()
	}
	wg.Wait()
}

func (b *RpcWork) runWorker(n int) {
	var throttle <-chan time.Time
	if b.QPS > 0 {
		throttle = time.Tick(time.Duration(1e6/(b.QPS)) * time.Microsecond)
	}



	for i := 0; i < n; i++ {
		// Check if application is stopped. Do not send into a closed channel.
		select {
		case <-b.stopCh:
			return
		default:
			if b.QPS > 0 {
				<-throttle
			}
			// b.makeRequest(client)
			b.makeRequest()
		}
	}
}

func makeRequestBody() *pb.HelloRequest {
	names := []string{"zhu", "bin", "hua"}
	index := rand.Intn(len(names) - 1)
	return &pb.HelloRequest{Name: names[index]}
}

func (b *RpcWork) makeRequest() {
	s := now()

	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewGreeterClient(conn)



	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(b.Timeout) * time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, makeRequestBody())
	r = r
	// if err != nil {
	//	log.Fatalf("could not greet: %v", err)
	// }
	// log.Printf("Greeting: %s", r.Message)

	t := now()
	finish := t - s
	b.results <- &result{
		offset:        s,
		statusCode:    200,
		duration:      finish,
		err:           err,
		contentLength: 100,
		connDuration:  1,
		dnsDuration:   1,
		reqDuration:   1,
		resDuration:   1,
		delayDuration: 1,
	}
}

func (b *RpcWork) Stop() {
	// Send stop signal so that workers can stop gracefully.
	for i := 0; i < b.C; i++ {
		b.stopCh <- struct{}{}
	}
}


func (b *RpcWork) Finish() {
	close(b.results)
	total := now() - b.start
	// Wait until the reporter is done.
	<-b.report.done
	b.report.finalize(total)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}