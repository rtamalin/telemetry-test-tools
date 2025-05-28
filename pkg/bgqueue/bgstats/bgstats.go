package bgstats

import (
	"math"
	"time"

	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue/bgjob"
)

// Background Queue Stats
type BgQueueStats struct {
	// private attributes
	started  time.Time
	finished time.Time

	// public attributes
	Total         int
	Completed     int
	Succeeded     int
	Failed        int
	Invalid       int
	ActiveTime    time.Duration
	AggregateTime time.Duration
	MinDuration   time.Duration
	MaxDuration   time.Duration
}

func New(totalJobs int) (bqs *BgQueueStats) {
	bqs = new(BgQueueStats)
	bqs.Init(totalJobs)
	return
}

func (bqs *BgQueueStats) Init(totalJobs int) {
	bqs.Total = totalJobs
}

func (bqs *BgQueueStats) Start() {
	bqs.started = time.Now()
}

func (bqs *BgQueueStats) Finish() {
	bqs.finished = time.Now()
	bqs.ActiveTime = bqs.finished.Sub(bqs.started)
}

func (bqs *BgQueueStats) Update(bj *bgjob.BackgroundJob) {
	// increment job completion state counters
	bqs.Completed += 1
	if bj.ExitStatus == 0 {
		bqs.Succeeded += 1
	} else if bj.ExitStatus > 0 {
		bqs.Failed += 1
	} else {
		bqs.Invalid += 1
	}

	// update duration tracking
	bqs.AggregateTime += bj.Duration

	if (bj.Duration < bqs.MinDuration) || (bqs.MinDuration == 0) {
		bqs.MinDuration = bj.Duration
	}

	if bj.Duration > bqs.MaxDuration {
		bqs.MaxDuration = bj.Duration
	}
}

func percentage(fraction, total float64) (pct float64) {
	roundingFactor := 1000.0
	pct = math.Round(((fraction / total) * 100 * roundingFactor)) / roundingFactor
	return
}

func (bqs *BgQueueStats) CompletionPercentage() float64 {
	return percentage(float64(bqs.Completed), float64(bqs.Total))
}

func (bqs *BgQueueStats) SuccessPercentage() float64 {
	return percentage(float64(bqs.Succeeded), float64(bqs.Total))
}

func (bqs *BgQueueStats) FailurePercentage() float64 {
	return percentage(float64(bqs.Failed), float64(bqs.Total))
}

func (bqs *BgQueueStats) InvalidPercentage() float64 {
	return percentage(float64(bqs.Failed), float64(bqs.Total))
}

func (bqs *BgQueueStats) CompletionRate() float64 {
	return 1.0 / (bqs.ActiveTime.Seconds() / float64(bqs.Completed))
}

func (bqs *BgQueueStats) AverageRunTime() time.Duration {
	return bqs.AggregateTime / time.Duration(bqs.Completed)
}

func (bqs *BgQueueStats) MinimumRunTime() time.Duration {
	return bqs.MinDuration
}

func (bqs *BgQueueStats) MaximumRunTime() time.Duration {
	return bqs.MaxDuration
}
