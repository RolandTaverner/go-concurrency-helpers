package collector

import (
	"context"
	"time"

	"github.com/RolandTaverner/go-concurrency-helpers/batch"
)

type ProducerFunc func(ctx context.Context) (interface{}, error)
type ConsumerFunc func(ctx context.Context, response interface{}, err error)

// ProducerConsumer holds producer and corresponding consumer function
// Output of Producer is input for Consumer
type ProducerConsumer struct {
	Producer ProducerFunc
	Consumer ConsumerFunc
}

// Collect runs Producers in concurrent goroutines, waits for completion, then for each Producer result runs Consumer.
func Collect(ctx context.Context, timeout time.Duration, handlers []ProducerConsumer) error {
	br := batch.New(uint(len(handlers)), 1, timeout)

	return br.Do(ctx,
		func(ctx context.Context, batch batch.Range) (interface{}, error) {
			producer := handlers[batch.From].Producer
			return producer(ctx)
		},
		func(ctx context.Context, batch batch.Range, response interface{}, err error) {
			consumer := handlers[batch.From].Consumer
			consumer(ctx, response, err)
		},
	)
}
