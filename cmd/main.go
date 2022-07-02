package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type stream string

const (
	streamStdout stream = "stdout"
	streamStderr stream = "stderr"
)

var errorRegex = regexp.MustCompile("error")

var (
	errorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "wrapped_binary_errors_total",
		Help: "The total number of errors encountered by the wrapped binary",
	})
)

func main() {
	//errorCounter.Add(0)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil) // TODO: Wait until server is started successfully before execing wrapped binary?? How? Needs some kind of loopback self-check...

	cmd := exec.Command("bash", "-c", `while :; do echo 'all good'; echo 'fatal error' 1>&2; echo 'debug statement' 1>&2; sleep 5; done`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	go readFromPipe(stdout, streamStdout)
	go readFromPipe(stderr, streamStderr)

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}

func readFromPipe(pipe io.ReadCloser, stream stream) {
	scanner := bufio.NewScanner(pipe)
	for {
		if ok := scanner.Scan(); !ok {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}

			return // EOF reached
		}

		line := scanner.Text()

		fmt.Println(stream, ":", line)
		if errorRegex.MatchString(line) {
			errorCounter.Inc()
			fmt.Println("\t", stream, ":", "Error encountered by wrapped binary!")
		}
	}
}
