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

// Purely for testing purposes.
func (a ReorderableList) Len() int           { return len(a) }
func (a ReorderableList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ReorderableList) Less(i, j int) bool { return a[i].GetKey().String() < a[j].GetKey().String() }

func (l ReorderableList) Insert(position uint) (*Key, error) {
	if position > uint(len(l)) {
		return nil, ErrOutOfBounds
	}

	if position == 0 {
		k, err := l.Prepend()
		if err != nil {
			return nil, err
		}
		return &k, nil
	}

	if position == uint(len(l)) {
		k, err := l.Append()
		if err != nil {
			return nil, err
		}
		return &k, nil
	}

	prev := l[position-1].GetKey()
	next := l[position].GetKey()

	for range 2 {
		k, err := Between(prev, next)
		if err == nil {
			return k, nil
		}

		l.rebalanceFrom(position, 1)

		// refresh prev/next keys
		prev = l[position-1].GetKey()
		next = l[position].GetKey()
	}

	return nil, ErrKeyInsertionFailedAfterRebalance
}

// Append does not change the size of the underlying list, but it may rebalance
// if necessary. It returns a new key which is ordered after the last item.
//
// In a worst case scenario, if the list already has a key at the maximum index,
// the list is rebalanced to make space at the end for the new generated key.
func (l ReorderableList) Append() (Key, error) {
	if len(l) == 0 {
		return Bottom, nil
	}

	for range 2 {
		last := l[len(l)-1].GetKey()
		k, err := Between(last, TopOf(last.bucket))
		if err == nil {
			return *k, nil
		}

		l.rebalanceFrom(uint(len(l)-1), -1)
	}

	return Key{}, ErrKeyInsertionFailedAfterRebalance
}

// Prepend does not change the size of the underlying list, but it may rebalance
// if necessary. It returns a new key which is ordered before the first item.
//
// Same worst case scenario as Append.
func (l ReorderableList) Prepend() (Key, error) {
	if len(l) == 0 {
		return Top, nil
	}

	for range 2 {
		first := l[0].GetKey()
		k, err := Between(BottomOf(first.bucket), first)
		if err == nil {
			return *k, nil
		}

		l.rebalanceFrom(0, 1)
	}

	return Key{}, ErrKeyInsertionFailedAfterRebalance
}

func (l ReorderableList) rebalanceFrom(position uint, direction int) error {
	ok := l.tryRebalanceFrom(position, direction)
	if ok {
		return nil
	}

	// If we're here, the worst case scenario was reached: every key is adjacent
	// to the next one. We need to normalise the entire list.

	return l.Normalize()
}

func (l ReorderableList) tryRebalanceFrom(position uint, direction int) bool {
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

			nextKey, err := Between(curr, next)
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
			next := l[i-1].GetKey()

			nextKey, err := Between(curr, next)
			if err == nil {
				l[i].SetKey(*nextKey)
				if i == int(position) {
					// first pass worked, can exit early.
					return true
				}
			}

			// If not OK, continue to rebalance forwards by shifting every key
		}
	}

	return false
}

// Normalize will distribute the keys evenly across the key space.
func (l ReorderableList) Normalize() error {
	for i := range l {
		f := float64(i+2) / float64(len(l)+3)
		b := l[i].GetKey().bucket

		nextKey, err := KeyAt(b, f)
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
