package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jhwbarlow/promwrap/pkg/config"
)

type regexCounterPair struct {
	counter prometheus.Counter
	regex   *regexp.Regexp
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	metricsAddr := flag.String("addr", ":2112", "colon separated address and port on which metrics server will bind")
	flag.Parse()

	childCmd, err := getChildCmd()
	if err != nil {
		log.Fatal("Getting child command: ", err)
	}

	configSourcer := config.NewYAMLFileConfigSourcer(*configPath)
	config, err := configSourcer.Config()
	if err != nil {
		log.Fatal("Sourcing configuration: ", err)
	}

	stdoutConfigPairs := createCounterPairs(config.Stdout)
	stderrConfigPairs := createCounterPairs(config.Stderr)
	bothStreamsConfigPairs := createCounterPairs(config.Both)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*metricsAddr, nil)
	// TODO: Wait until server is started successfully before execing wrapped binary?? How? Needs some kind of loopback self-check...
	// For now, we hackily sleep a little
	time.Sleep(500 * time.Millisecond)

	cmd := exec.Command(childCmd[0], childCmd[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("Getting child command stdout pipe: ", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal("Getting child command stderr pipe: ", err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan)

	if err := cmd.Start(); err != nil {
		log.Fatal("Starting child command: ", err)
	}

	// Propagate all signals to child
	go passSignalToChild(signalChan, cmd.Process)

	// Read child stdout and stderr and look for counters to increment
	go scanStream(stdout, os.Stdout, append(stdoutConfigPairs, bothStreamsConfigPairs...))
	go scanStream(stderr, os.Stderr, append(stderrConfigPairs, bothStreamsConfigPairs...))

	cmd.Wait()                           // No need to check for errors as we just want to return whatever exit code the child process provided
	os.Exit(cmd.ProcessState.ExitCode()) // However go will return -1/255 if the child terminated due to signal, we could make this bash-like and return 128 + $sig_num instead
}

func getChildCmd() ([]string, error) {
	args := os.Args[1:]

	childCmdStartIndex := 0
	for i, arg := range args {
		if arg == "--" {
			childCmdStartIndex = i + 1
			break
		}
	}

	// If the start index is still zero, no '--' was present
	if childCmdStartIndex == 0 {
		return nil, errors.New("No '--' present in arguments")
	}

	// If nothing follows the '--', then no child command was specified
	fmt.Println(childCmdStartIndex, len(args))
	if childCmdStartIndex == len(args) {
		return nil, errors.New("No child command specified in arguments")
	}

	return args[childCmdStartIndex:], nil
}

func createCounterPairs(configs []*config.CounterConfig) []*regexCounterPair {
	counterPairs := make([]*regexCounterPair, 0, len(configs))

	for _, config := range configs {
		counter := promauto.NewCounter(prometheus.CounterOpts{
			Name: config.CounterName,
			Help: config.Help,
		})

		regex := regexp.MustCompile(config.Regex)

		counterPair := &regexCounterPair{counter, regex}
		counterPairs = append(counterPairs, counterPair)
	}

	return counterPairs
}

func passSignalToChild(signalChan <-chan os.Signal, child *os.Process) {
	child.Signal(<-signalChan)
}

func scanStream(in io.Reader, out io.Writer, counterPairs []*regexCounterPair) {
	scanner := bufio.NewScanner(in)
	for {
		if ok := scanner.Scan(); !ok {
			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}

			return // EOF reached
		}

		line := scanner.Text()

		fmt.Fprintln(out, line)
		for _, counterPair := range counterPairs {
			if counterPair.regex.MatchString(line) {
				counterPair.counter.Inc()
			}
		}
	}
}
