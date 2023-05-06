package gobench

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/zeromicro/go-zero/core/timex"
)

const defaultAddr = "localhost:8081"

type (
	Metrics struct {
		Median time.Duration
		P99    time.Duration
	}

	Bench struct {
		records   map[int]Metrics
		startTime time.Duration
		current   time.Duration
		bucket    taskHeap
	}

	Config struct {
		Times    int
		Duration time.Duration
	}
)

func NewBench() *Bench {
	return &Bench{
		records:   make(map[int]Metrics),
		startTime: timex.Now(),
		current:   timex.Now(),
	}
}

func (b *Bench) Run(config Config, fn func()) {
	if config.Times == 0 && config.Duration == 0 {
		log.Fatal("either times or duration should be set")
	}

	if config.Times == 0 {
		config.Times = math.MaxInt
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	var timeCh <-chan time.Time
	if config.Duration > 0 {
		timeCh = time.After(config.Duration)
	}

	i := 0
	for i < config.Times {
		select {
		case <-timeCh:
			goto chart
		case <-c:
			goto chart
		default:
			b.runSingle(fn)
			i++
		}
	}

chart:
	fmt.Printf("run times: %d\n", i)

	signal.Stop(c)
	go func() {
		time.Sleep(time.Second)
		openBrowser("http://" + defaultAddr)
	}()

	http.HandleFunc("/", generateChart(b.records))
	http.ListenAndServe(defaultAddr, nil)
}

func (b *Bench) runSingle(fn func()) {
	start := timex.Now()
	fn()
	elapsed := timex.Since(start)

	if timex.Since(b.current) > time.Second {
		metrics := calculate(b.bucket)
		index := int((b.current - b.startTime) / time.Second)
		b.records[index] = metrics
		b.current = start
		b.bucket = taskHeap{}
	}

	b.bucket.Push(Task{
		Duration: elapsed,
	})
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Println(err)
	}
}
