package bgqueue

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue/bgjob"
	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue/bgstats"
)

// Background Job Management
type bgJobSlot struct{}

// Background Queue
type BgQueue struct {
	// private attributes
	numJobs    int
	numSlots   int
	prefix     string
	jobSlots   chan bgJobSlot
	jobResults chan *bgjob.BackgroundJob

	// progress tracking
	progress ProgressCallback
	numTicks int
	nextTick int
	tickStep int

	// public attributes
	Command    []string
	JobGroup   *sync.WaitGroup
	Jobs       []*bgjob.BackgroundJob
	QueueStart time.Time
	Stats      *bgstats.BgQueueStats
}

func New(totalJobs, jobSlots int, prefix string, command []string) (bq *BgQueue) {
	bq = new(BgQueue)
	bq.Init(totalJobs, jobSlots, prefix, command)
	return
}

func (bq *BgQueue) updateNextTick() {
	bq.nextTick = min(bq.nextTick+bq.tickStep, bq.numJobs)
}

func (bq *BgQueue) setTicks(numTicks int) {
	bq.numTicks = numTicks
	bq.tickStep = int(math.Ceil(float64(bq.numJobs) / float64(bq.numTicks)))
	bq.nextTick = 0
	bq.updateNextTick()
}

func (bq *BgQueue) Init(totalJobs, jobSlots int, prefix string, command []string) {
	// record the parameters
	bq.numJobs = totalJobs
	bq.numSlots = jobSlots
	bq.prefix = prefix
	bq.Command = command

	// set the default progress callback and progress ticks
	bq.progress = Progress
	bq.setTicks(20)

	// init the stats
	bq.Stats = bgstats.New(totalJobs)

	// allocate a channel with options.Batch jobSlots to act as a semaphore
	bq.jobSlots = make(chan bgJobSlot, bq.numSlots)

	// allocate a channel with sufficient entries to hold options.Total
	// BackgroundJob results
	bq.jobResults = make(chan *bgjob.BackgroundJob, bq.numJobs)

	// allocate a sync.WaitGroup to track background jobs completions
	bq.JobGroup = new(sync.WaitGroup)

	bq.Jobs = make([]*bgjob.BackgroundJob, bq.numJobs)
	for i := range bq.Jobs {
		bq.Jobs[i] = bgjob.New(i, "bgjob", bq.Command)
	}
}

func (bq *BgQueue) runJob(
	bj *bgjob.BackgroundJob,
	slot chan bgJobSlot, // semaphore to limit concurrency
	resultQueue chan<- *bgjob.BackgroundJob, // Queue (channel) where results are sent
) {
	// ensure WaitGroup is signalled when routine finishes
	defer bq.JobGroup.Done()

	// job is ready to run
	bj.MarkReady()

	// acquire a job slot from the semaphore and ensure we release it when
	// the routine finishes
	slot <- bgJobSlot{}
	defer func() { <-slot }()

	// job has now started
	bj.MarkStarted()

	// run the command
	if err := bj.Run(); err != nil {
		var cmdError string
		if bj.CmdError != nil {
			cmdError = bj.CmdError.Error()
		}
		slog.Warn(
			"Job failed",
			slog.Int("id", bj.Id),
			slog.String("name", bj.Name),
			slog.Any("command", bj.Command),
			slog.Int("exitStatus", bj.ExitStatus),
			slog.String("cmdError", cmdError),
			slog.String("jobError", bj.Error.Error()),
			slog.String("error", err.Error()),
		)
	}

	// complete this routine, sending job result back to the main goroutine
	resultQueue <- bj
}

func (bq *BgQueue) Run() {
	bq.Stats.Start()
	bq.QueueStart = time.Now()

	// create bq.numJobs go routines to run the background jobs,
	// limiting how many are actively running bq.numSlots
	for i := range bq.Jobs {
		// track that we are starting a new job
		bq.JobGroup.Add(1)

		// create a go routine to run the background job
		go bq.runJob(bq.Jobs[i], bq.jobSlots, bq.jobResults)
	}

	// wait for background jobs to complete
	go func() {
		bq.JobGroup.Wait()
		close(bq.jobResults)
	}()

	for bj := range bq.jobResults {
		bq.Stats.Update(bj)
		if bq.Stats.Completed >= bq.nextTick {
			bq.updateNextTick()
			if bq.progress != nil {
				bq.progress(bq, bj)
			}
		}
		if false {
			slog.Info(
				"Progress",
				slog.Float64("completed%", bq.Stats.CompletionPercentage()),
				slog.Float64("fail%", bq.Stats.FailurePercentage()),
				slog.Float64("success%", bq.Stats.SuccessPercentage()),
			)
		}
	}
	bq.Stats.Finish()
}

// progress callback type
type ProgressCallback func(bq *BgQueue, bj *bgjob.BackgroundJob)

func Progress(bq *BgQueue, bj *bgjob.BackgroundJob) {
	fmt.Printf(
		"Progress: complete=%6d(%6.2f%%) fail=%6d(%6.2f%%) success=%6d(%6.2f%%)\n",
		bq.Stats.Completed,
		bq.Stats.CompletionPercentage(),
		bq.Stats.Failed,
		bq.Stats.FailurePercentage(),
		bq.Stats.Succeeded,
		bq.Stats.SuccessPercentage(),
	)
}
