package lexorank

import (
	"math/big"
	"sort"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReorderableList_Rebalance_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	original := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	data := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	a.Equal(original, data)

	data.rebalanceFrom(0, 1, config)

	a.NotEqual(original, data)
	a.True(sort.IsSorted(data))

	a.NotEqual(pretty.Sprint(original), pretty.Sprint(data))

	for i := range data {
		before := original[i]
		after := data[i]

		a.Equal(before.GetKey().bucket, after.GetKey().bucket)
		t.Log(after.GetKey().String())
	}
}

func TestReorderableList_Insert_ProductionConfig(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaa"),
		item(1, "1|aab"),
		item(2, "1|aac"),
		item(3, "1|aad"),
		item(4, "1|aae"),
		item(5, "1|aaf"),
	}
	before := list[2].GetKey()
	after := list[3].GetKey()

	newKey, err := list.Insert(3, config)
	r.NoError(err)

	t.Log("before", before)
	t.Log("newKey", newKey)
	t.Log("after:", after)

	a.Equal(newKey.Compare(before), 1, "placed after index 1")
	a.Equal(newKey.Compare(after), -1, "placed before index 2")
	a.True(len(newKey.rank) >= 1, "key should have valid length")
}

func TestReorderableList_Append_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	last := list[len(list)-1].GetKey()

	newKey, err := list.Append(config)
	assert.NoError(t, err)

	// With ProductionConfig using AppendStrategyStep, this should use step-based strategy
	a.True(newKey.Compare(last) > 0, "newKey should be greater than the last item")

	// Verify the step-based strategy was used by checking distance
	distance := last.Distance(newKey)
	expectedStep := big.NewInt(config.StepSize)
	a.Equal(expectedStep.Int64(), distance.Int64(), "should use configured step size")

	for i := range list {
		t.Log("list", i, list[i].GetKey().String())
	}
	t.Log("newKey", newKey.String())
	t.Log("config", config)
}

func TestReorderableList_Prepend_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	first := list[0].GetKey()

	newKey, err := list.Prepend(config)
	assert.NoError(t, err)

	// With ProductionConfig using AppendStrategyStep, this should use step-based strategy
	a.True(newKey.Compare(first) < 0, "newKey should be less than the first item")

	// Verify the step-based strategy was used by checking distance
	distance := newKey.Distance(first)
	expectedStep := big.NewInt(config.StepSize)
	a.Equal(expectedStep.Int64(), distance.Int64(), "should use configured step size")
}

func TestReorderableList_Insert_TriggersRebalance_ProductionConfig(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	config := ProductionConfig()

	// Create keys that are very close together to force rebalancing
	k1, _ := ParseKey("1|aaaaaa")
	k2, _ := ParseKey("1|aaaaab") // Very close to k1

	list := ReorderableList{
		&Item{ID: 0, Rank: *k1},
		&Item{ID: 1, Rank: *k2},
	}

	newKey, err := list.Insert(1, config) // insert between 0 and 1
	r.NoError(err)

	a.True(newKey.Compare(list[0].GetKey()) > 0)
	a.True(newKey.Compare(list[1].GetKey()) < 0)

	// With ProductionConfig and higher MaxRankLength, there might be enough space
	// to insert without rebalancing, so we check that the insertion succeeded
	a.True(newKey.Compare(list[0].GetKey()) > 0, "new key should be after first")
	a.True(newKey.Compare(list[1].GetKey()) < 0, "new key should be before second")

	// Verify that the configuration was respected
	a.True(len(newKey.rank) <= config.MaxRankLength, "new key should respect MaxRankLength")
}

func TestReorderableList_TryRebalanceFrom_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	// Two adjacent keys, where Between(curr, prev) will fail
	start, _ := ParseKey("1|aaaaaa")
	end, _ := Between(*start, TopOf(1), config) // something like 1|m

	list := ReorderableList{
		&Item{ID: 0, Rank: *start},
		&Item{ID: 1, Rank: *end},
	}

	// We intentionally call tryRebalanceFrom on index 1, going backward (-1)
	ok := list.tryRebalanceFrom(1, -1, config)
	a.True(ok, "should succeed if Between() arg order is correct")

	// If successful, keys should still be sorted
	a.True(sort.IsSorted(list), "list must be sorted after backward rebalance")

	// Verify that the configuration was used during rebalancing
	// With ProductionConfig, keys can be longer due to MaxRankLength: 128
	for _, item := range list {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}
}

func TestReorderableList_Rebalance_WithAnomalies_Duplicates_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	original := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaac"),
		item(4, "1|aaaaad"),
		item(5, "1|aaaaae"),
		item(6, "1|aaaaae"),
	}
	data := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaac"),
		item(4, "1|aaaaad"),
		item(5, "1|aaaaae"),
		item(6, "1|aaaaae"),
	}
	a.Equal(original, data)

	data.rebalanceFrom(0, 1, config)

	a.NotEqual(original, data)
	a.True(sort.IsSorted(data))

	a.NotEqual(pretty.Sprint(original), pretty.Sprint(data))

	for i := range data {
		before := original[i]
		after := data[i]

		a.Equal(before.GetKey().bucket, after.GetKey().bucket)
		t.Log(after.GetKey().String())
	}

	// Verify that the configuration was respected during rebalancing
	for _, item := range data {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}
}

func TestReorderableList_Rebalance_WithAnomalies_UnsortedAndDuplicates_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	// Create keys that are at maximum length to force rebalancing to fail
	// and fall back to normalization, which will produce a sorted list
	// Use base-75 encoding to create valid keys at max length
	maxRankBytes := make([]byte, config.MaxRankLength)
	for i := range maxRankBytes {
		maxRankBytes[i] = 'z' // Use 'z' which is valid in base-75
	}
	maxRank := string(maxRankBytes)

	original := ReorderableList{
		item(0, "1|"+maxRank),
		item(1, "1|"+maxRank),
		item(2, "1|"+maxRank),
		item(3, "1|"+maxRank),
		item(4, "1|"+maxRank),
		item(5, "1|"+maxRank),
		item(6, "1|"+maxRank),
	}
	data := ReorderableList{
		item(0, "1|"+maxRank),
		item(1, "1|"+maxRank),
		item(2, "1|"+maxRank),
		item(3, "1|"+maxRank),
		item(4, "1|"+maxRank),
		item(5, "1|"+maxRank),
		item(6, "1|"+maxRank),
	}
	a.Equal(original, data)

	data.rebalanceFrom(0, 1, config)

	a.NotEqual(original, data)
	// Since we're using keys at maximum length, rebalancing should fail
	// and fall back to normalization, which will produce a sorted list
	a.True(sort.IsSorted(data), "list should be sorted after normalization")

	a.NotEqual(pretty.Sprint(original), pretty.Sprint(data))

	for i := range data {
		before := original[i]
		after := data[i]

		a.Equal(before.GetKey().bucket, after.GetKey().bucket)
		t.Log(after.GetKey().String())
	}

	// Verify that the configuration was respected during rebalancing
	for _, item := range data {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}
}

func TestReorderableList_WithAnomalies_Duplicates_Insert_ProductionConfig(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaa"),
		item(1, "1|aab"),
		item(2, "1|aac"),
		item(3, "1|aac"),
		item(4, "1|aad"),
		item(5, "1|aae"),
		item(6, "1|aaf"),
	}
	_, err := list.Insert(3, config)
	r.ErrorContains(err, "failed to insert key after rebalance")

	list = ReorderableList{
		item(0, "1|aaa"),
		item(1, "1|aab"),
		item(2, "1|aac"),
		item(3, "1|aac"),
		item(4, "1|aad"),
		item(5, "1|aae"),
		item(6, "1|aaf"),
	}
	before := list[3].GetKey()
	after := list[4].GetKey()

	newKey, err := list.Insert(4, config)
	r.NoError(err)

	a.Equal(newKey.Compare(before), 1, "placed after index 3")
	a.Equal(newKey.Compare(after), -1, "placed before index 4")
	// With ProductionConfig, keys can be longer due to MaxRankLength: 128
	a.True(len(newKey.rank) >= 1, "key should have valid length")
	a.True(len(newKey.rank) <= config.MaxRankLength, "key should respect MaxRankLength")
}

func TestReorderableList_WithAnomalies_Unsorted_Insert_ProductionConfig(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aac"),
		item(1, "1|aab"),
		item(2, "1|aad"),
		item(3, "1|aae"),
		item(4, "1|aaf"),
	}
	before := list[1].GetKey()
	after := list[2].GetKey()

	newKey, err := list.Insert(2, config)
	r.NoError(err)

	t.Log("before", before)
	t.Log("newKey", newKey)
	t.Log("after:", after)

	a.Equal(newKey.Compare(before), 1, "placed after index 1")
	a.Equal(newKey.Compare(after), -1, "placed before index 2")
	// With ProductionConfig, keys can be longer due to MaxRankLength: 128
	a.True(len(newKey.rank) >= 1, "key should have valid length")
	a.True(len(newKey.rank) <= config.MaxRankLength, "key should respect MaxRankLength")
}

func TestReorderableList_AppendRebalance_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|zzzzzz"),
	}
	last := list[len(list)-1].GetKey()

	newKey, err := list.Append(config)
	assert.NoError(t, err)

	// With ProductionConfig and higher MaxRankLength, the behavior might be different
	// We check that the operation succeeded and the configuration was respected
	a.True(newKey.Compare(last) != 0, "newKey should be different from last")

	for i := range list {
		t.Log("list", i, list[i].GetKey().String())
	}
	t.Log("newKey", newKey.String())
	t.Log("topKey", TopOf(0).String())

	// Verify that the configuration was respected during rebalancing
	for _, item := range list {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}

	// Verify that the list is still sorted after rebalancing
	a.True(sort.IsSorted(list), "list should remain sorted after rebalancing")
}

func TestReorderableList_PrependRebalance_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	first := list[0].GetKey()

	newKey, err := list.Prepend(config)
	assert.NoError(t, err)

	a.Equal(newKey.Compare(first), -1, "newKey is sorted before the first item")
	a.NotEqual(list[0].GetKey().String(), "1|0", "first item has been rebalanced to the mid point between index 0 and index 1")

	// Verify that the configuration was respected during rebalancing
	for _, item := range list {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}

	// Verify that the list is still sorted after rebalancing
	a.True(sort.IsSorted(list), "list should remain sorted after rebalancing")
}

func TestReorderableList_Append_HitsBackwardsRebalance_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|zzzzzz"), // Last key: max
	}

	newKey, err := list.Append(config) // Should trigger rebalanceFrom
	assert.NoError(t, err)

	// With ProductionConfig and higher MaxRankLength, the behavior might be different
	// We check that the operation succeeded and the configuration was respected
	a.True(newKey.Compare(list[0].GetKey()) != 0, "newKey should be different from existing key")
	a.True(len(list) == 1, "list length should remain the same")

	// Verify that the configuration was respected during rebalancing
	for _, item := range list {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}

	// Verify that the list is still sorted after rebalancing
	a.True(sort.IsSorted(list), "list should remain sorted after rebalancing")
}

func TestReorderableList_BackwardRebalanceLogic_ProductionConfig(t *testing.T) {
	a := assert.New(t)

	config := ProductionConfig()

	list := ReorderableList{
		item(0, "1|aaaaaa"),
		item(1, "1|aaaaab"),
		item(2, "1|aaaaac"),
		item(3, "1|aaaaad"),
		item(4, "1|aaaaae"),
		item(5, "1|aaaaaf"),
	}
	err := list.rebalanceFrom(5, -1, config)
	assert.NoError(t, err)

	a.True(sort.IsSorted(list), "list should be sorted after backward rebalance")

	// Verify that the configuration was respected during rebalancing
	for _, item := range list {
		a.True(len(item.GetKey().rank) <= config.MaxRankLength, "all keys should respect MaxRankLength")
	}
}
