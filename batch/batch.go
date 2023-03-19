package batch

import (
	"context"
	"errors"
	"time"
)

var (
	ErrEmptyBatch = errors.New("empty batch")
	ErrTimedOut   = errors.New("request timed out")
	ErrUnexpected = errors.New("unexpected channel close") // Should never happen
)

type Range struct {
	From, Count uint
}

type Response struct {
	BatchRange Range
	Response   interface{}
	Error      error
}

type Executor func(ctx context.Context, batchRange Range) (interface{}, error)
type Processor func(ctx context.Context, batchRange Range, response interface{}, err error)

type Batch struct {
	TotalCount uint
	BatchSize  uint
	Timeout    time.Duration
}

// New creates and returns batch instance
// totalCount - total elements count
// batchSize - desired elements count in single batch. Size of the last batch = totalCount % batchSize (if totalCount % batchSize != 0)
// timeout - amount of time Batch will wait for producer
func New(totalCount uint, batchSize uint, timeout time.Duration) *Batch {
	return &Batch{
		TotalCount: totalCount,
		BatchSize:  batchSize,
		Timeout:    timeout,
	}
}

// Do runs goroutines for every batch, waits for producer's completion, then runs consumers
// ctx - context
// batchExecutor - function that accepts batch range and executes work
// batchProcessor - function that accepts single batch results
// Do returns error if 1 or more executors were not completed in time. In this case processors for timed out executors do not called,
func (b *Batch) Do(ctx context.Context, batchExecutor Executor, batchProcessor Processor) error {
	if b.TotalCount == 0 {
		return ErrEmptyBatch
	}

	batches := makeBatchRanges(b.TotalCount, b.BatchSize)
	batchesCount := len(batches)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, b.Timeout)
	defer cancel()

	respCh := make(chan *Response, batchesCount)

	for _, batchRange := range batches {
		go func(ctx context.Context, batchRange Range, sink chan<- *Response) {
			ctxBatch, cancelBatch := context.WithTimeout(ctx, b.Timeout)
			defer cancelBatch()

			response, err := batchExecutor(ctxBatch, batchRange)
			select {
			case <-ctxBatch.Done():
				return
			default:
				sink <- &Response{
					BatchRange: batchRange,
					Response:   response,
					Error:      err,
				}
			}
		}(ctx, batchRange, respCh)
	}

	results := make([]*Response, 0, len(batches))
	var err error = nil

	for len(results) < batchesCount {
		select {
		case <-ctxWithTimeout.Done():
			err = ErrTimedOut
			break
		case resp, ok := <-respCh:
			if !ok {
				return ErrUnexpected
			}
			results = append(results, resp)
		}
		if err != nil {
			break
		}
	}

	for _, r := range results {
		batchProcessor(ctxWithTimeout, r.BatchRange, r.Response, r.Error)
	}

	return err
}

func makeBatchRanges(totalCount, batchSize uint) []Range {
	if batchSize == 0 {
		batchSize = totalCount
	}
	batches := make([]Range, 0, totalCount/batchSize+1)

	currentBatchFrom := uint(0)
	for {
		b := Range{From: currentBatchFrom}
		if b.From+batchSize >= totalCount {
			b.Count = totalCount - b.From
		} else {
			b.Count = batchSize
		}
		batches = append(batches, b)
		if b.From+b.Count >= totalCount {
			break
		}
		currentBatchFrom = b.From + b.Count
	}

	return batches
}
