package bgstats

import (
	"math"
	"time"

	"github.com/rtamalin/telemetry-test-tools/pkg/bgqueue/bgjob"
	"golang.org/x/exp/constraints"
)

// Background Queue Stats
type BgQueueStats struct {
	// private attributes
	started   time.Time
	finished  time.Time
	durations []time.Duration

	// public attributes
	Total      int
	Completed  int
	Succeeded  int
	Failed     int
	Invalid    int
	ActiveTime time.Duration
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

	bqs.durations = append(bqs.durations, bj.Duration)
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

func (bqs *BgQueueStats) AggregateRunTime() time.Duration {
	return sum(bqs.durations)
}

func (bqs *BgQueueStats) AverageRunTime() time.Duration {
	return average(bqs.durations)
}

func (bqs *BgQueueStats) MinimumRunTime() time.Duration {
	return min(bqs.durations)
}

func (bqs *BgQueueStats) MaximumRunTime() time.Duration {
	return max(bqs.durations)
}

func (bqs *BgQueueStats) StdDev() time.Duration {
	return fromSecond(math.Sqrt(bqs.Variance().Seconds()))
}

func (bqs *BgQueueStats) RootMeanSquare() time.Duration {
	return fromSecond(math.Sqrt(average(squared(toSeconds(bqs.durations)))))
}

type numeric interface {
	constraints.Signed | constraints.Unsigned | constraints.Float
}

func sum[T numeric](vals []T) T {
	var tot T
	for _, v := range vals {
		tot += v
	}
	return tot
}

func max[T numeric](vals []T) T {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func min[T numeric](vals []T) T {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func average[T numeric](vals []T) T {
	return sum(vals) / T(len(vals))
}

func squared[T numeric](vals []T) []T {
	//sqd := make([]T, len(vals))
	//sqd := make([]T, 0)
	var sqd []T
	for _, v := range vals {
		sqd = append(sqd, v*v)
	}
	return sqd
}

func toSeconds(durations []time.Duration) []float64 {
	//seconds := make([]float64, len(durations))
	//seconds := make([]float64, 0)
	var seconds []float64
	for _, d := range durations {
		seconds = append(seconds, d.Seconds())
	}
	return seconds
}

func fromSecond(second float64) time.Duration {
	return time.Duration(second * float64(time.Second))
}

func fromSeconds(seconds []float64) []time.Duration {
	//durations := make([]time.Duration, len(seconds))
	//durations := make([]time.Duration, 0)
	var durations []time.Duration
	for _, s := range seconds {
		durations = append(durations, fromSecond(s))
	}
	return durations
}

func (bqs *BgQueueStats) Variance() time.Duration {
	avg := average(toSeconds(bqs.durations))
	//deltas := make([]float64, len(bqs.durations))
	//deltas := make([]float64, 0)
	var deltas []float64
	for _, d := range toSeconds(bqs.durations) {
		deltas = append(deltas, d-avg)
	}

	squaredDeltas := fromSeconds(squared(deltas))

	return average(squaredDeltas)
}
