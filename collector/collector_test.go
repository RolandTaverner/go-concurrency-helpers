package collector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeIntProducer(res int) ProducerFunc {
	return func(ctx context.Context) (interface{}, error) {
		return res, nil
	}
}

func makeStringProducer(res string) ProducerFunc {
	return func(ctx context.Context) (interface{}, error) {
		return res, nil
	}
}

type info struct {
	i1, i2, i3 int
	s1, s2, s3 string
}

func Test_makeBatches(t *testing.T) {
	res := info{}

	err := Collect(
		context.Background(),
		time.Second,
		[]ProducerConsumer{
			{
				Producer: makeIntProducer(1),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(int)
					require.True(t, ok)
					res.i1 = n
				},
			},
			{
				Producer: makeIntProducer(2),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(int)
					require.True(t, ok)
					res.i2 = n
				},
			},
			{
				Producer: makeIntProducer(3),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(int)
					require.True(t, ok)
					res.i3 = n
				},
			},
			{
				Producer: makeStringProducer("abc"),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(string)
					require.True(t, ok)
					res.s1 = n
				},
			},
			{
				Producer: makeStringProducer("def"),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(string)
					require.True(t, ok)
					res.s2 = n
				},
			},
			{
				Producer: makeStringProducer("ghi"),
				Consumer: func(ctx context.Context, response interface{}, err error) {
					n, ok := response.(string)
					require.True(t, ok)
					res.s3 = n
				},
			},
		},
	)

	require.NoError(t, err)

	expected := info{
		i1: 1,
		i2: 2,
		i3: 3,
		s1: "abc",
		s2: "def",
		s3: "ghi",
	}

	require.Equal(t, expected, res)
}
