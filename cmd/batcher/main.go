package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var appStart time.Time

type BackgroundJob struct {
	Id         int           // job id
	Name       string        // job name
	Created    time.Time     // time at which the job was created
	Started    time.Time     // time at which the job started
	Duration   time.Duration // how long the job took to run
	Stdout     string        // stdout, if any, from the job
	Stderr     string        // stderr, if any, from the job
	ExitStatus int           // the exit status of the job itself
	Error      error         // the error returned from the exec.Cmd.Run()
	CmdError   error         // the error found in exec.Cmd.Err
}

func (bj *BackgroundJob) printBanner(subFmt string, subArgs ...any) {
	args := append([]any{bj.Name}, subArgs...)

	fmt.Printf("[%s "+subFmt+"]\n", args...)
}

// Print BackgroundJob results
func (bj *BackgroundJob) Print() {
	// calculate time deltas
	jobCreatedDelta := bj.Created.Sub(appStart)
	jobStartDelta := bj.Started.Sub(bj.Created)

	if bj.Error != nil {
		bj.printBanner("job failed")
		fmt.Printf("Command: %s\n", strings.Join(options.Command, " "))
		fmt.Printf("Error: %s\n", bj.Error.Error())
		if bj.CmdError != nil {
			fmt.Printf("Cmd.Err: %s", bj.CmdError)
		}

		return
	}

	if bj.Stdout != "" {
		bj.printBanner("job stdout")
		fmt.Printf("%s", bj.Stdout)
		// append a newline if the string doesn't end with one
		if !strings.HasSuffix(bj.Stdout, "\n") {
			fmt.Println()
		}
	} else {
		bj.printBanner("job stdout empty")
	}

	if bj.Stderr != "" {
		bj.printBanner("job stderr")
		fmt.Printf("%s", bj.Stderr)
		// append a newline if the string doesn't end with one
		if !strings.HasSuffix(bj.Stderr, "\n") {
			fmt.Println()
		}
	} else {
		bj.printBanner("job stderr empty")
	}

	bj.printBanner(
		"job times: +% 10.6fs -> +% 10.6fs (% 10.6fs)",
		jobCreatedDelta.Seconds(),
		jobStartDelta.Seconds(),
		bj.Duration.Seconds(),
	)
	bj.printBanner("job exit status %d", bj.ExitStatus)
}

// Options
type Options struct {
	Total   int
	Batch   int
	Prefix  string
	Command []string
}

// default option values
var option_defaults = Options{
	Total:  50,
	Batch:  10,
	Prefix: "bgjob",
	Command: []string{
		"sleep",
		"1",
	},
}

// options that will be specified
var options Options

// Background Job Management
type jobSlot struct{}

func runBackgroundJob(
	jobId int,
	slot chan jobSlot, // semaphore to limit concurrency
	jobGroup *sync.WaitGroup, // WaitGroup for completion signalling
	resultQueue chan<- BackgroundJob, // Queue (channel) where results are sent
) {
	// create the job, recording the creation time
	job := BackgroundJob{
		Id:      jobId,
		Created: time.Now(),
	}

	// generate the job name
	job.Name = fmt.Sprintf("%s_%06d", options.Prefix, jobId)

	// ensure WaitGroup is signalled when routine finishes
	defer jobGroup.Done()

	// acquire a job slot from the semaphore and ensure we release it when
	// the routine finishes
	slot <- jobSlot{}
	defer func() { <-slot }()

	// record the time that the job started running
	job.Started = time.Now()

	// create a Command struct to manage running the command, using
	// Buffers for the stdout and stderr
	cmd := exec.Command(options.Command[0], options.Command[1:]...)
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	// run the specified command and wait for it to complete,
	// recording the time at which it finished
	job.Error = cmd.Run()
	job.Duration = time.Now().Sub(job.Started)

	// record the stdout and stderr
	job.Stdout = cmd.Stdout.(*bytes.Buffer).String()
	job.Stderr = cmd.Stderr.(*bytes.Buffer).String()

	// extract the exit status if the job command ran but failed
	if job.Error != nil {
		if exitError, ok := job.Error.(*exec.ExitError); ok {
			job.ExitStatus = exitError.ExitCode()
			job.Error = nil
		} else {
			// otherwise there was an error with the command itself, so
			// set the exit status to -1
			job.ExitStatus = -1
		}
		if cmd.Err != nil {
			job.CmdError = cmd.Err
		}
	}

	// complete this routine, sending job result back to the main goroutine
	resultQueue <- job
}

func main() {
	// define flags for our options
	flag.IntVar(&options.Total, "total", option_defaults.Total, "The `Total` number of jobs to run")
	flag.IntVar(&options.Batch, "batch", option_defaults.Batch, "Up to `Batch` jobs will run at the same time")
	flag.StringVar(&options.Prefix, "prefix", option_defaults.Prefix, "The `Prefix` to use when generating the jobs name")

	// parse the command line
	flag.Parse()

	// validate options
	if options.Total <= 0 {
		slog.Error("Total must be >= 0")
		flag.Usage()
		os.Exit(1)
	}

	if options.Batch <= 0 {
		slog.Error("Batch must be >= 0")
		flag.Usage()
		os.Exit(1)
	}

	if options.Batch > options.Total {
		slog.Error("Batch must be <= Total", slog.Int("Batch", options.Batch), slog.Int("Total", options.Total))
		flag.Usage()
		os.Exit(1)
	}

	if len(options.Prefix) < 2 {
		slog.Error("Prefix must be at least 2 characters")
		flag.Usage()
		os.Exit(1)
	}

	// use the remaining command line args as the shell command to be run
	remainingArgs := flag.Args()
	if len(remainingArgs) > 0 {
		options.Command = remainingArgs
	} else {
		options.Command = option_defaults.Command
	}

	// allocate a channel with options.Batch jobSlots to act as a semaphore
	jobSlots := make(chan jobSlot, options.Batch)

	// allocate a channel with sufficient entries to hold options.Total
	// BackgroundJob results
	jobResults := make(chan BackgroundJob, options.Total)

	// allocate a sync.WaitGroup to track background jobs completions
	jobGroup := new(sync.WaitGroup)

	appStart = time.Now()
	slog.Info("Starting background jobs", slog.Int("Batch", options.Batch), slog.Int("Total", options.Total))

	// create options.Total go routines to run the background jobs,
	// limiting how many actually run at a given time to options.Batch
	for jobId := 0; jobId < options.Total; jobId++ {
		// track that we are starting a new job
		jobGroup.Add(1)

		// create a go routine to run the background job
		go runBackgroundJob(jobId+1, jobSlots, jobGroup, jobResults)
	}

	go func() {
		jobGroup.Wait()
		close(jobResults)
	}()

	var totalDuration time.Duration = 0
	var minDuration time.Duration = 0
	var maxDuration time.Duration = 0
	var completed int = 0
	for job := range jobResults {
		job.Print()
		completed += 1
		slog.Info("Progress", slog.Int("Remaining", options.Total-completed))

		totalDuration += job.Duration

		if (job.Duration < minDuration) || (minDuration == 0) {
			minDuration = job.Duration
		}

		if job.Duration > maxDuration {
			maxDuration = job.Duration
		}
	}

	totalTime := time.Now().Sub(appStart).Seconds()
	slog.Info(
		"Times",
		slog.Float64("Total Time (s)", totalTime),
		slog.Float64("Completion Rate (job/s)", 1.0/(totalTime/float64(options.Total))),
		slog.Float64("Average Job Duration (s)", totalDuration.Seconds()/float64(options.Total)),
		slog.Float64("Min Job Duration (s)", minDuration.Seconds()),
		slog.Float64("Max Job Duration (s)", maxDuration.Seconds()),
	)
}
