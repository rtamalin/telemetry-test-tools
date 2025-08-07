package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue"
	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue/bgjob"
)

var appStart time.Time

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
		"echo",
		"Hello",
		"World",
	},
}

// options that will be specified
var options Options

// Background Job Management
type jobSlot struct{}

func runBackgroundJob(
	job *bgjob.BackgroundJob,
	slot chan jobSlot, // semaphore to limit concurrency
	jobGroup *sync.WaitGroup, // WaitGroup for completion signalling
	resultQueue chan<- *bgjob.BackgroundJob, // Queue (channel) where results are sent
) {
	// ensure WaitGroup is signalled when routine finishes
	defer jobGroup.Done()

	// create the job, recording the creation time
	//job := NewBackgroundJob(jobId, "bgjob", options.Command)

	// job is ready to run
	job.MarkReady()

	// acquire a job slot from the semaphore and ensure we release it when
	// the routine finishes
	slot <- jobSlot{}
	defer func() { <-slot }()

	// record the time that the job started running
	job.Started = time.Now()

	// create a Command struct to manage running the command, using
	// Buffers for the stdout and stderr
	cmd := exec.Command(job.Command[0], job.Command[1:]...)
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	// run the specified command and wait for it to complete,
	// recording the time at which it finished
	job.Error = cmd.Run()
	job.Duration = time.Since(job.Started)

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

	bgQueue := bgqueue.New(
		options.Total,
		options.Batch,
		"bgjob",
		options.Command,
	)

	bgQueue.Run()

	for _, job := range bgQueue.Jobs {
		job.Print(false)
	}

	bgqStats := bgQueue.Stats
	slog.Debug(
		"Times",
		slog.Float64("Completion Rate (job/s)", bgqStats.CompletionRate()),
		slog.Float64("Active (Wallclock) Time (s)", bgqStats.ActiveTime.Seconds()),
		slog.Float64("Aggregate Time (s)", bgqStats.AggregateRunTime().Seconds()),
		slog.Float64("Average Job Duration (s)", bgqStats.AverageRunTime().Seconds()),
		slog.Float64("Min Job Duration (s)", bgqStats.MinimumRunTime().Seconds()),
		slog.Float64("Max Job Duration (s)", bgqStats.MaximumRunTime().Seconds()),
	)

	fmt.Printf("Summary:\n")
	fmt.Printf("  %-23s: %10d\n", "Total Jobs", options.Total)
	fmt.Printf("  %-23s: %10d\n", "Batch Size", options.Batch)
	fmt.Printf("  %-23s: %10.3f %%\n", "Success Rate", bgqStats.SuccessPercentage())
	fmt.Printf("  %-23s: %10.3f jobs/s\n", "Completion Rate", bgqStats.CompletionRate())
	fmt.Printf("  %-23s: %10.3f s\n", "Active (Wallclock) Time", bgqStats.ActiveTime.Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Aggregate Job Time", bgqStats.AggregateRunTime().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Average Job Time", bgqStats.AverageRunTime().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Minimum Job Time", bgqStats.MinimumRunTime().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Maximum Job Time", bgqStats.MaximumRunTime().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Job Time Variance", bgqStats.Variance().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Job Time StdDev", bgqStats.StdDev().Seconds())
	fmt.Printf("  %-23s: %10.3f s\n", "Job Time RMS", bgqStats.RootMeanSquare().Seconds())
}
