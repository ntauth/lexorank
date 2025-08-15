# Lexorank

A Go implementation of Lexorank for efficient list sorting and reordering.

## What is Lexorank?

Lexorank is a sorting key system that allows you to insert items between any two existing items without reordering the entire list. It's perfect for drag-and-drop interfaces, task management systems, and any application where users need to reorder items frequently.

## Features

- **Efficient Insertions**: Insert items between any two positions without reordering
- **Automatic Rebalancing**: Keys are automatically redistributed when space runs out
- **Variable Precision**: Keys can grow in length to accommodate more items
- **Configurable Strategies**: Choose how new keys are generated when appending/prepending
- **Database Ready**: Implements SQL driver interfaces for easy database integration

## Core Concepts

### Lexorank Keys

A lexorank key consists of:
- **Bucket**: A namespace identifier (0, 1, 2)
- **Rank**: A string that determines the sort order

### How It Works

1. **Initial Setup**: Items get evenly distributed keys across the key space
2. **Insertion**: New items get keys between existing ones
3. **Rebalancing**: When keys get too long, the system redistributes them evenly
4. **Precision**: Keys can grow in length to accommodate more items

## Usage

### Basic Operations

```go
package main

import "github.com/ntauth/lexorank"

// Create a list of items
items := []lexorank.Reorderable{
    &MyItem{key: lexorank.Key{raw: []byte("0|aaaaaa"), rank: []byte("aaaaaa"), bucket: 0}},
    &MyItem{key: lexorank.Key{raw: []byte("0|zzzzzz"), rank: []byte("zzzzzz"), bucket: 0}},
}

// Create a reorderable list
list := lexorank.NewReorderableList(items, lexorank.DefaultConfig())

// Insert a new item at position 1
newKey, err := list.Insert(1)
if err != nil {
    panic(err)
}

// Append an item to the end
newKey, err = list.Append()
if err != nil {
    panic(err)
}
```

### Configuration

Configure how the system behaves:

```go
// Default configuration (between-based strategy)
config := lexorank.DefaultConfig()

// Production configuration (step-based strategy)
config := lexorank.ProductionConfig().
    WithAppendStrategy(lexorank.AppendStrategyStep).
    WithStepSize(1000)

// Custom configuration
config := lexorank.DefaultConfig().
    WithMaxRankLength(12).
    WithStepSize(500)
```

### Append Strategies

Choose how new keys are generated:

- **Default Strategy**: Uses `Between(last, TopOf(bucket))` for predictable spacing
- **Step Strategy**: Uses `After(stepSize)` for append, `Before(stepSize)` for prepend

```go
// Step-based strategy for predictable spacing
config := lexorank.ProductionConfig().
    WithAppendStrategy(lexorank.AppendStrategyStep).
    WithStepSize(1000)

list := lexorank.NewReorderableList(items, config)

// Append will use last.After(1000)
// Prepend will use first.Before(1000)
newKey, err := list.AppendWithConfig(config)
```

## Why big.Int?

Lexorank fundamentally involves finding integers between other integers. As keys grow longer, these integers can exceed fixed-size types:

- **Base 75, length 6**: `75^6 ≈ 17.7 trillion` (fits in `int64`)
- **Base 75, length 8**: `75^8 ≈ 9.8 quintillion` (exceeds `int64`)
- **Base 75, length 12**: `75^12 ≈ 5.4 × 10^22` (way beyond `int64`)

Using `big.Int` ensures mathematical correctness for any key length without overflow.

## Database Integration

Keys implement SQL driver interfaces for easy database storage:

```go
// Store in database
key := lexorank.Key{raw: []byte("0|aaaaaa"), rank: []byte("aaaaaa"), bucket: 0}
db.Exec("INSERT INTO items (rank_key) VALUES (?)", key)

// Load from database
var key lexorank.Key
db.QueryRow("SELECT rank_key FROM items WHERE id = ?", id).Scan(&key)
```

## Performance

- **Insertions**: O(1) average case, O(n) worst case (when rebalancing)
- **Rebalancing**: Automatically triggered when needed
- **Memory**: Keys are compact byte arrays
- **Precision**: Grows automatically as needed
