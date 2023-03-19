# Introduction

The goal of this package is to help with
- concurrent batch processing
- running concurrent tasks

The typical solution to running concurrent code is [WaitGroup](https://gobyexample.com/waitgroups), but developer still needs to
- implement error handling
- implement timeouts
- remember that he can't use RW access to shared data in concurrent goroutines

This library solves these problems by splitting processing in two phases - produce and consume.
In produce phase you do data processing and return result. In consume phase you consume result.
Producers run concurrently in own goroutines, consumers run sequentially in the same goroutine.
So in producer you can't modify shared data (and usually you do not need), in consumer you can modify shared data without synchronization.


# Usage

## Batch

`batch.Batch` is intended to process data in batches.

General use of `batch.Batch` looks like this:
```go
    data := make([]SomeData)
    // fill data ...
    
	batchErr := batch.New(uint(len(data)), 10 /*batch size*/, time.Second).Do(ctx,
		func(ctx context.Context, batch batch.Range) (interface{}, error) {
			for _, dataElement := range data[batch.From : batch.From+batch.Count] {
                // process dataElement
			}
			result := &MyBatchResultType{}
            // fill batch result ...
            
			return result, nil
		},
		func(ctx context.Context, batchRange batch.Range, batchResult interface{}, err error) {
			result, ok := batchResult.(*MyBatchResultType)
			if !ok {
				panic("unexpected error - producer return type should be *MyBatchResultType")
			}
            // Handle result
		},
	)
	if batchErr != nil {
        // handle error
	}
```

### Example 1
Assume you have some data and you want to split it in batches and process in parallel.

The following code block demonstrates how to compute sum of array elements concurrently
```go
import (
    "context"
    "fmt"
    "time"

    "github.com/RolandTaverner/go-concurrency-helpers/batch"
)

// BatchProcessExample demonstrates how to use batch.Batch to compute sum of array elements in parallel
func BatchProcessExample() {
	ctx := context.Background()

	// Mock data
	data := make([]int, 0, 100)
	for i := 0; i < 100; i++ {
		data[i] = i
	}

	// Sum of all data[] elements
	totalSum := 0

	batchErr := batch.New(uint(len(data)), 10, time.Second).Do(ctx,
		func(ctx context.Context, batch batch.Range) (interface{}, error) {
			batchSum := 0
			for _, n := range data[batch.From : batch.From+batch.Count] {
				batchSum += n
			}
			return batchSum, nil
		},
		func(ctx context.Context, batchRange batch.Range, batchResult interface{}, err error) {
			batchSum, ok := batchResult.(int)
			if !ok {
				panic("unexpected error - producer return type should be int")
			}
			// Batch consumers executed in the same goroutine - so we can modify shared data, no need to synchronize here
			totalSum += batchSum
		},
	)
	if batchErr != nil {
		fmt.Printf(batchErr.Error())
		return
	}

	fmt.Printf("Sum=%d", totalSum)
}
```

### Example 2

This is more real-life example. Assume you have remote service that accepts array of item IDs and returns items.
The service limits count of items in single request by, for example, 10, while you want more.
So we make parallel requests to service with up to 10 items in each request.

```go
func BatchProcessRemoteCallExample() {
	ctx := context.Background()
	remoteItemService := &itemServiceMock{}

	// Mock data
	itemIDs := make([]int, 0, 123)
	for i := 0; i < 100; i++ {
		itemIDs[i] = i
	}

	itemsMap := make(map[int]Item)

	batchErr := batch.New(uint(len(itemIDs)), 10, time.Second).Do(ctx,
		func(ctx context.Context, batch batch.Range) (interface{}, error) {
			itemsToQuery := itemIDs[batch.From : batch.From+batch.Count]
			return remoteItemService.GetItems(ctx, itemsToQuery)
		},
		func(ctx context.Context, batchRange batch.Range, batchResult interface{}, err error) {
			items, ok := batchResult.([]Item)
			if !ok {
				panic("unexpected error - producer return type should be []Item")
			}
			// Here we can check which items were not returned by service and log this, for example
			expectedItemIDs := itemIDs[batchRange.From : batchRange.From+batchRange.Count]
			// ...

			// Batch consumers executed in the same goroutine - so we can modify shared data, no need to synchronize here
			for _, i := range items {
				itemsMap[i.ID] = i
			}
		},
	)
	if batchErr != nil {
		fmt.Printf(batchErr.Error())
	}

	// Here we have itemsMap filled
	// Some requests may fail or time out, so it may contain fewer items than requested
}

type Item struct {
	ID       int
	ItemData string
}

type itemServiceMock struct {
}

// GetItems returns items by IDs. 10 is the max items IDs in single request
// Assume it calls remote service which has such limitations
func (s *itemServiceMock) GetItems(ctx context.Context, itemIDs []int) ([]Item, error) {
	if len(itemIDs) > 10 {
		return nil, errors.New("bad request: limit exceeded")
	}

	// Return mock data
	result := make([]Item, 0, len(itemIDs))
	for _, id := range itemIDs {
		result = append(result, Item{ID: id, ItemData: strconv.FormatInt(int64(id), 10)})
	}
	return result, nil
}
```

## Collector

TODO 