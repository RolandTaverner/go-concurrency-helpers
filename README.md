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

This package can be helpful for Go beginners interested to study how to handle multiple channels and respect timeouts.

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
package example

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
package example

import (
	"context"
	"fmt"
	"time"

	"github.com/RolandTaverner/go-concurrency-helpers/batch"
)

func BatchProcessRemoteCallExample() {
	ctx := context.Background()
	remoteItemService := &itemServiceMock{}

	// Mock data
	itemIDs := make([]int, 0, 123)
	for i := 0; i < 100; i++ {
		itemIDs[i] = i
	}

	// Will store all received items here
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

`collector.Collect()` function intended to run any code concurrently, but it is essentially useful when you need to call several remote services.
`collector.Collect()` implemented on top of the `batch.Batch` so it runs producers concurrently and consumers sequentially.

Assume you need to call 3 services - UserProfile, UserAccount and UserPurchases returning data by user ID.
The code block below demonstrates how to call 3 services concurrently.
 
```go
package example

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RolandTaverner/go-concurrency-helpers/collector"
)

func CollectExample(ctx context.Context) error {
	// Mock services
	userProfileService := &userProfileServiceMock{}
	userAccountService := &userAccountServiceMock{}
	userPurchasesService := &userPurchasesServiceMock{}

	// Mock data
	userID := 123

	// Variables to store results
	var userProfile *UserProfile
	var userProfileErr error

	var userAccount *UserAccount
	var userAccountErr error

	var userPurchases *UserPurchases
	var userPurchasesErr error

	// Run 3 concurrent calls to remote services with 500ms timeout
	err := collector.Collect(ctx, time.Millisecond*500,
		[]collector.ProducerConsumer{
			{
				Producer: func(ctx context.Context) (interface{}, error) {
					return userProfileService.GetUserProfile(ctx, userID)
				},
				Consumer: func(ctx context.Context, response interface{}, err error) {
					userProfileResponse, ok := response.(*UserProfile)
					if !ok {
						// Should never happen
						panic("unexpected error - producer return type should be *UserProfile")
					}
					userProfile = userProfileResponse
					userProfileErr = err
				},
			},
			{
				Producer: func(ctx context.Context) (interface{}, error) {
					return userAccountService.GetUserAccount(ctx, userID)
				},
				Consumer: func(ctx context.Context, response interface{}, err error) {
					userAccountResponse, ok := response.(*UserAccount)
					if !ok {
						// Should never happen
						panic("unexpected error - producer return type should be *UserAccount")
					}
					userAccount = userAccountResponse
					userAccountErr = err
				},
			},
			{
				Producer: func(ctx context.Context) (interface{}, error) {
					return userPurchasesService.GetUserPurchases(ctx, userID)
				},
				Consumer: func(ctx context.Context, response interface{}, err error) {
					userPurchasesResponse, ok := response.(*UserPurchases)
					if !ok {
						// Should never happen
						panic("unexpected error - producer return type should be *UserPurchases")
					}
					userPurchases = userPurchasesResponse
					userPurchasesErr = err
				},
			},
		})
	if err != nil {
		// 1 or more services timed out
		// If you need all or nothing, can just return an error here
		return errors.New("something went wrong")
	}

	// Handle errors
	if userProfileErr != nil {
		// Service error
	} else if userProfile == nil {
		// Service did not respond in time
	}
	if userAccountErr != nil {
		// Service error
	} else if userAccount == nil {
		// Service did not respond in time
	}
	if userPurchasesErr != nil {
		// Service error
	} else if userPurchases == nil {
		// Service did not respond in time
	}

	// Do something with userProfile, userAccount and userPurchases
	return nil
}

type UserProfile struct {
	ID    int
	Name  string
	Email string
}

type userProfileServiceMock struct{}

func (s *userProfileServiceMock) GetUserProfile(ctx context.Context, userID int) (*UserProfile, error) {
	return &UserProfile{
		ID:    userID,
		Name:  fmt.Sprintf("User %d", userID),
		Email: fmt.Sprintf("user_%d@example.com", userID),
	}, nil
}

type UserAccount struct {
	ID             int
	AccountBalance float64
}

type userAccountServiceMock struct{}

func (s *userAccountServiceMock) GetUserAccount(ctx context.Context, userID int) (*UserAccount, error) {
	return &UserAccount{
		ID:             userID,
		AccountBalance: float64(userID) * 3.141,
	}, nil
}

type UserPurchases struct {
	ID             int
	PurchasedItems []int
}

type userPurchasesServiceMock struct{}

func (s *userPurchasesServiceMock) GetUserPurchases(ctx context.Context, userID int) (*UserPurchases, error) {
	return &UserPurchases{
		ID:             userID,
		PurchasedItems: []int{1, 2, 3},
	}, nil
}
```