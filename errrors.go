package lexorank

import "fmt"

var ErrOutOfBounds = fmt.Errorf("out of bounds")
var ErrRebalanceRequired = fmt.Errorf("rebalance required")
var ErrKeyInsertionFailedAfterRebalance = fmt.Errorf("failed to insert key after rebalance")
