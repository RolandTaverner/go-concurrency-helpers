package batch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testBatch struct {
	totalCount, batchSize uint
	expected              []Range
}

func Test_makeBatches(t *testing.T) {
	cases := []testBatch{
		{
			totalCount: 10,
			batchSize:  3,
			expected: []Range{
				{0, 3},
				{3, 3},
				{6, 3},
				{9, 1},
			},
		},
		{
			totalCount: 10,
			batchSize:  5,
			expected: []Range{
				{0, 5},
				{5, 5},
			},
		},
		{
			totalCount: 10,
			batchSize:  10,
			expected: []Range{
				{0, 10},
			},
		},
		{
			totalCount: 10,
			batchSize:  100,
			expected: []Range{
				{0, 10},
			},
		},
		{
			totalCount: 3,
			batchSize:  1,
			expected: []Range{
				{0, 1},
				{1, 1},
				{2, 1},
			},
		},
		{
			totalCount: 123,
			batchSize:  0,
			expected: []Range{
				{0, 123},
			},
		},
	}

	for _, c := range cases {
		require.Equal(t, c.expected, makeBatchRanges(c.totalCount, c.batchSize))
	}
}

func Test_BatchRequest_Do(t *testing.T) {
	sumIn := uint64(0)
	items := make([]uint, 0, 100000)
	for i := uint(0); i < 100000; i++ {
		items = append(items, i)
		sumIn += uint64(i)
	}

	respItems := make([]uint, 0, len(items))

	br := New(uint(len(items)), 321, time.Second*10)

	err := br.Do(context.Background(),
		func(ctx context.Context, batch Range) (interface{}, error) {
			batchResp := make([]uint, 0)
			for _, n := range items[batch.From : batch.From+batch.Count] {
				batchResp = append(batchResp, n*10)
			}
			return batchResp, nil
		},
		func(ctx context.Context, batch Range, resp interface{}, err error) {
			require.NoError(t, err)

			batchResp, ok := resp.([]uint)
			assert.True(t, ok)
			require.Equal(t, uint(batch.Count), uint(len(batchResp)))

			respItems = append(respItems, batchResp...)
		},
	)
	require.NoError(t, err)
	require.Equal(t, len(items), len(respItems))

	sumOut := uint64(0)
	for _, n := range respItems {
		sumOut += uint64(n)
	}
	require.Equal(t, sumIn*10, sumOut)
}

func Test_BatchRequest_Do_Fail(t *testing.T) {
	sumIn := uint64(0)
	items := make([]uint, 0, 10)
	for i := uint(0); i < 10; i++ {
		items = append(items, i)
		sumIn += uint64(i)
	}

	respItems := make([]uint, 0, len(items))

	br := New(uint(len(items)), 3, time.Second*10)

	err := br.Do(context.Background(),
		func(ctx context.Context, batch Range) (interface{}, error) {
			if batch.From == 3 {
				return nil, errors.New("fail")
			}
			batchResp := make([]uint, 0)
			for _, n := range items[batch.From : batch.From+batch.Count] {
				batchResp = append(batchResp, n*10)
			}
			return batchResp, nil
		},
		func(ctx context.Context, batch Range, resp interface{}, err error) {
			if batch.From == 3 {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			batchResp, ok := resp.([]uint)
			assert.True(t, ok)
			require.Equal(t, uint(batch.Count), uint(len(batchResp)))

			respItems = append(respItems, batchResp...)
		},
	)
	require.NoError(t, err)
	require.Equal(t, len(items)-3, len(respItems))
}

func Test_BatchRequest_Do_Timeout(t *testing.T) {
	sumIn := uint64(0)
	items := make([]uint, 0, 1000)
	for i := uint(0); i < 1000; i++ {
		items = append(items, i)
		sumIn += uint64(i)
	}

	respItems := make([]uint, 0, len(items))

	br := New(uint(len(items)), 3, time.Second)

	err := br.Do(context.Background(),
		func(ctx context.Context, batch Range) (interface{}, error) {
			if batch.From == 3 {
				time.Sleep(time.Second * 2)
			}
			batchResp := make([]uint, 0)
			for _, n := range items[batch.From : batch.From+batch.Count] {
				batchResp = append(batchResp, n*10)
			}
			return batchResp, nil
		},
		func(ctx context.Context, batch Range, resp interface{}, err error) {
			if batch.From == 3 {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			batchResp, ok := resp.([]uint)
			assert.True(t, ok)
			require.Equal(t, uint(batch.Count), uint(len(batchResp)))

			respItems = append(respItems, batchResp...)
		},
	)
	require.Error(t, err)
	require.Equal(t, ErrTimedOut, err)
}
