package lexorank

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKey_Between_ProductionConfig(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	cfg := ProductionConfig()

	lhs, err := ParseKey("0|a")
	r.NoError(err)
	rhs, err := ParseKey("0|b")
	r.NoError(err)

	maxIterationsBeforeRebalancingIsRequired := 890
	for i := range maxIterationsBeforeRebalancingIsRequired {
		mid, err := Between(*lhs, *rhs, cfg)
		if i == maxIterationsBeforeRebalancingIsRequired-1 {
			t.Log("final mid point	", lhs.String())
			r.ErrorIs(err, ErrRebalanceRequired)
		} else {
			r.NoError(err)
			a.True(mid.Compare(*lhs) > 0 && mid.Compare(*rhs) < 0, "mid should be between lhs and rhs")
			lhs = mid
		}
	}
}

func TestKey_Between_BottomAndTop_ProductionConfig(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	cfg := ProductionConfig()

	lhs := BottomOf(0)
	rhs := TopOf(0)

	maxIterationsBeforeRebalancingIsRequired := 897
	for i := range maxIterationsBeforeRebalancingIsRequired {
		mid, err := Between(lhs, rhs, cfg)
		if i == maxIterationsBeforeRebalancingIsRequired-1 {
			t.Log("final mid point	", lhs.String())
			r.ErrorIs(err, ErrRebalanceRequired)
		} else {
			r.NoError(err)
			a.True(mid.Compare(lhs) > 0 && mid.Compare(rhs) < 0, "mid should be between lhs and rhs")
			lhs = *mid
		}
	}
}
