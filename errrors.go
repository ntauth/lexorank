package lexorank

import "github.com/pkg/errors"

var (
	ErrOutOfBounds                      = errors.New("out of bounds")
	ErrRebalanceRequired                = errors.New("rebalance required")
	ErrNormalizationRequired            = errors.New("normalization required")
	ErrKeyInsertionFailedAfterRebalance = errors.New("failed to insert key after rebalance")
)
