package collector

import (
	"context"
	"time"

	"github.com/RolandTaverner/go-concurrency-helpers/batch"
)

type ProducerFunc func(ctx context.Context) (interface{}, error)
type ConsumerFunc func(ctx context.Context, response interface{}, err error)

type ProducerConsumer struct {
	Producer ProducerFunc
	Consumer ConsumerFunc
}

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
