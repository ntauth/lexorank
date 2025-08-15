package lexorank

type Orderable interface {
	GetKey() Key
}

type Mutable interface {
	SetKey(Key)
}

type Reorderable interface {
	Orderable
	Mutable
}

// ReorderableList represents a collection of orderable items, usually from a
// database. It's designed so that you read a range of items from your storage
// that you wish to apply one or more re-order operations to before saving them
// back in bulk. It supports automatic re-balancing of the keys if a between key
// goes beyond the advised length limit for the default lexorank length of 6.
//
// The Reorderable interface describes a type that supports mutating its own key
// in order to facilitate moving items or re-balancing the list.
//
// Rebalancing does not necessarily mean a write to every item, as the inline
// rebalance algorithm can operate on a small amount of neighbour items before
// falling back to normalising the entire list if it deems necessary.
//
// Reorderable list is assumed to be already ordered upon instantiation.
type ReorderableList []Reorderable

// Config holds the configuration for this list
type ReorderableListConfig struct {
	*Config
}

// NewReorderableList creates a new ReorderableList with the given configuration
func NewReorderableList(items []Reorderable, config *Config) ReorderableList {
	return ReorderableList(items)
}

// DefaultReorderableList creates a new ReorderableList with default configuration
func DefaultReorderableList(items []Reorderable) ReorderableList {
	return NewReorderableList(items, DefaultConfig())
}

// Purely for testing purposes.
func (a ReorderableList) Len() int           { return len(a) }
func (a ReorderableList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ReorderableList) Less(i, j int) bool { return a[i].GetKey().String() < a[j].GetKey().String() }

func (l ReorderableList) Insert(position uint, config *Config) (*Key, error) {
	if position > uint(len(l)) {
		return nil, ErrOutOfBounds
	}

	if position == 0 {
		k, err := l.Prepend(config)
		if err != nil {
			return nil, err
		}
		return &k, nil
	}

	if position == uint(len(l)) {
		k, err := l.Append(config)
		if err != nil {
			return nil, err
		}
		return &k, nil
	}

	prev := l[position-1].GetKey()
	next := l[position].GetKey()

	for range 2 {
		k, err := Between(prev, next, config)
		if err == nil {
			return k, nil
		}

		l.rebalanceFrom(position, 1, config)

		// refresh prev/next keys
		prev = l[position-1].GetKey()
		next = l[position].GetKey()
	}

	return nil, ErrKeyInsertionFailedAfterRebalance
}

// Append does not change the size of the underlying list, but it may rebalance
// if necessary. It returns a new key which is ordered after the last item using the
// specified configuration for append strategy.
func (l ReorderableList) Append(config *Config) (Key, error) {
	if len(l) == 0 {
		return BottomOf(0), nil
	}

	for range 2 {
		last := l[len(l)-1].GetKey()
		k, err := SmartAppend(last, config)
		if err == nil {
			return *k, nil
		}

		l.rebalanceFrom(uint(len(l)-1), -1, config)
	}

	return Key{}, ErrKeyInsertionFailedAfterRebalance
}

// Prepend does not change the size of the underlying list, but it may rebalance
// if necessary. It returns a new key which is ordered before the first item using the
// specified configuration.
func (l ReorderableList) Prepend(config *Config) (Key, error) {
	if len(l) == 0 {
		return TopOf(0), nil
	}

	for range 2 {
		first := l[0].GetKey()
		k, err := SmartPrepend(first, config)
		if err == nil {
			return *k, nil
		}

		l.rebalanceFrom(0, 1, config)
	}

	return Key{}, ErrKeyInsertionFailedAfterRebalance
}

func (l ReorderableList) rebalanceFrom(position uint, direction int, config *Config) error {
	ok := l.tryRebalanceFrom(position, direction, config)
	if ok {
		return nil
	}

	// If we're here, the worst case scenario was reached: every key is adjacent
	// to the next one. We need to normalise the entire list.

	return l.Normalize(config)
}

func (l ReorderableList) tryRebalanceFrom(position uint, direction int, config *Config) bool {
	if direction > 0 && position >= uint(len(l)-1) {
		return false // at end of list
	}
	if direction < 0 && position == 0 {
		return false // at start of list
	}

	if direction > 0 {
		for i := int(position); i < len(l)-1; i++ {
			curr := l[i].GetKey()
			next := l[i+1].GetKey()

			nextKey, err := Between(curr, next, config)
			if err == nil {
				l[i+1].SetKey(*nextKey)
				if i == int(position) {
					// first pass worked, can exit early.
					return true
				}
			}

			// If not OK, continue to rebalance forwards by shifting every key
		}
	} else {
		for i := int(position); i > 0; i-- {
			curr := l[i].GetKey()
			prev := l[i-1].GetKey()

			// For backward rebalancing, we need prev < curr, so swap arguments
			nextKey, err := Between(prev, curr, config)
			if err == nil {
				l[i].SetKey(*nextKey)
				if i == int(position) {
					// first pass worked, can exit early.
					return true
				}
			}

			// If not OK, continue to rebalance backwards by shifting every key
		}
	}

	return false
}

// Normalize will distribute the keys evenly across the key space
// using the specified configuration for precision settings.
func (l ReorderableList) Normalize(config *Config) error {
	if !config.AutoNormalize {
		return ErrNormalizationRequired
	}

	for i := range l {
		f := float64(i+2) / float64(len(l)+3)
		b := l[i].GetKey().bucket

		nextKey, err := KeyAt(b, f, config)
		if err != nil {
			return err
		}

		l[i].SetKey(nextKey)
	}

	return nil
}

func (l ReorderableList) IsSorted() bool {
	for i := 1; i < len(l); i++ {
		if l[i-1].GetKey().Compare(l[i].GetKey()) >= 0 {
			return false
		}
	}
	return true
}
