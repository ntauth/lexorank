package lexorank

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"

	"github.com/pkg/errors"
)

const (
	Minimum  = '0'
	Midpoint = 'U'
	Maximum  = 'z'
)

var (
	Bottom = Key{
		raw:    []byte{'0', '|', Minimum},
		rank:   []byte{Minimum},
		bucket: 0,
	}
	Top = Key{
		raw:    []byte{'0', '|', Maximum, Maximum, Maximum, Maximum, Maximum, Maximum},
		rank:   []byte{Maximum, Maximum, Maximum, Maximum, Maximum, Maximum},
		bucket: 0,
	}
	Middle = Key{
		raw:    []byte{'0', '|', Midpoint, Midpoint, Midpoint, Midpoint, Midpoint, Midpoint},
		rank:   []byte{Midpoint, Midpoint, Midpoint, Midpoint, Midpoint, Midpoint},
		bucket: 0,
	}

	// Charset for encoding positions
	charset  = []byte("0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz")
	maxValue = int(math.Pow(float64(len(charset)), float64(6))) // full keyspace

)

func TopOf(b uint8) Key {
	return Key{
		raw:    []byte{byte(b + '0'), '|', Maximum, Maximum, Maximum, Maximum, Maximum, Maximum},
		rank:   []byte{Maximum, Maximum, Maximum, Maximum, Maximum, Maximum},
		bucket: b,
	}
}

func BottomOf(b uint8) Key {
	return Key{
		raw:    []byte{byte(b + '0'), '|', Minimum},
		rank:   []byte{Minimum},
		bucket: b,
	}
}

func MiddleOf(b uint8) Key {
	return Key{
		raw:    []byte{byte(b + '0'), '|', Midpoint, Midpoint, Midpoint, Midpoint, Midpoint, Midpoint},
		rank:   []byte{Midpoint, Midpoint, Midpoint, Midpoint, Midpoint, Midpoint},
		bucket: b,
	}
}

const (
	keyLength  = 8 // the full key length "0|aaaaaa"
	rankLength = 6 // the part after the |: "aaaaaa"
)

type Key struct {
	raw    []byte // "0|aaaaaa"
	rank   Rank   // "aaaaaa"
	bucket uint8  // 0, 1, 2
}

func (k Key) String() string {
	return string(k.raw)
}

func (k Key) GoString() string {
	return string(k.raw)
}

func (k Key) Compare(b Key) int {
	return bytes.Compare(k.raw, b.raw)
}

func (k *Key) SetBucket(b uint8) {
	if b > 2 {
		b = 0
	}
	k.bucket = b
}

type Keys []Key

type Rank []byte

func (r Rank) minAt(i int) byte {
	if i >= len(r) {
		return Minimum
	}

	return r[i]
}

func (r Rank) maxAt(i int) byte {
	if i >= len(r) {
		return Maximum
	}

	return r[i]
}

func ParseKey(s string) (*Key, error) {
	if len(s) > keyLength {
		return nil, fmt.Errorf("invalid key length: %d", len(s))
	}

	bucket, err := strconv.Atoi(string(s[0]))
	if err != nil {
		return nil, err
	}

	rank := []byte(s[2:])

	return parseRaw(uint8(bucket), rank)
}

func parseRaw(bucket uint8, rank []byte) (*Key, error) {
	if len(rank) > rankLength {
		return nil, fmt.Errorf("invalid rank length: %d", len(rank))
	}

	for _, b := range rank {
		if b < Minimum || b > Maximum {
			return nil, fmt.Errorf("invalid byte value: %c", b)
		}
	}

	raw := append([]byte{byte(bucket + 48), '|'}, rank...)

	return &Key{
		raw:    raw,
		rank:   rank,
		bucket: bucket,
	}, nil
}

// KeyAt generates a key from a specific numeric position in the key space.
func KeyAt(bucket uint8, f float64) (Key, error) {
	bucketChar := byte(bucket + 48)

	base := float64(len(charset)) // 75
	key := make([]byte, 0, rankLength)

	for i := 0; i < rankLength; i++ {
		f *= base
		index := int(f)
		if index >= len(charset) {
			index = len(charset) - 1
		}
		key = append(key, charset[index])
		f -= float64(index)

		if f <= 0.0 {
			break
		}
	}

	k, err := ParseKey(string(append([]byte{bucketChar, '|'}, key...)))
	if err != nil {
		return Key{}, err
	}

	return *k, nil
}

// Between returns a new key that is between the current key and the second key.
// If the boolean return value is false, it indicates keys are getting too long
// and thus a rebalance is required. "Too long" is very subjective. The limit
// set in this library is 6 which gives you around 400k worst case re-orders.
func Between(lhs, rhs Key) (*Key, error) {
	if lhs.Compare(rhs) > 0 {
		return Between(rhs, lhs)
	}

	midKey := &Key{
		raw:    []byte{},
		rank:   Rank{},
		bucket: lhs.bucket,
	}

	for i := 0; ; i++ {
		prevChar := lhs.rank.minAt(i)
		nextChar := rhs.rank.maxAt(i)

		if prevChar == nextChar {
			midKey.rank = append(midKey.rank, prevChar)
			continue
		}

		m, ok := mid(prevChar, nextChar)
		if !ok {
			midKey.rank = append(midKey.rank, prevChar)
			continue
		}

		midKey.rank = append(midKey.rank, m)
		break
	}

	valid := len(midKey.rank) <= rankLength
	if !valid {
		return nil, ErrRebalanceRequired
	}

	if string(midKey.rank) >= string(rhs.rank) {
		return nil, ErrRebalanceRequired
	}

	// ASCII representation of bucket value, ranges from 0-2 so 1 basic addition works fine
	bucketChar := lhs.bucket + 48

	midKey.raw = append(midKey.raw, []byte{bucketChar, '|'}...)
	midKey.raw = append(midKey.raw, midKey.rank...)

	return midKey, nil
}

func (k Key) After(distance int64) (*Key, error) {
	index, err := decodeBase75(k.rank)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode rank")
	}
	next := encodeBase75(index + distance)

	n, err := parseRaw(k.bucket, next)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse next key")
	}

	return n, nil
}

func (k Key) Before(distance int64) (*Key, error) {
	index, err := decodeBase75(k.rank)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode rank")
	}
	if index-distance < 0 {
		return nil, ErrOutOfBounds
	}

	next := encodeBase75(index - distance)

	n, err := parseRaw(k.bucket, next)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse next key")
	}

	return n, nil
}

func mid(a, b byte) (byte, bool) {
	if a == b {
		return a, false
	}

	m := (a + b) / 2

	if m == a || m == b {
		return a, false
	}

	return m, true
}

func decodeBase75(rank []byte) (int64, error) {
	var index int64
	for _, c := range rank {
		pos := int64(bytes.IndexByte(charset, c))
		if pos == -1 {
			return 0, errors.New("invalid character in rank")
		}
		index = index*int64(len(charset)) + pos
	}
	return index, nil
}

func encodeBase75(val int64) []byte {
	if val == 0 {
		return []byte{charset[0]}
	}

	var out []byte
	for val > 0 {
		rem := val % int64(len(charset))
		out = append([]byte{charset[rem]}, out...)
		val = val / int64(len(charset))
	}

	return out
}

func Random() (Key, error) {
	f := rand.Float64()
	return KeyAt(0, f)
}

var (
	_ encoding.TextMarshaler   = (*Key)(nil)
	_ encoding.TextUnmarshaler = (*Key)(nil)
	_ json.Marshaler           = (*Key)(nil)
	_ json.Unmarshaler         = (*Key)(nil)
	_ driver.Valuer            = (*Key)(nil)
	_ sql.Scanner              = (*Key)(nil)
)

// TextMarshaler
func (k Key) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

// TextUnmarshaler
func (k *Key) UnmarshalText(text []byte) error {
	parsed, err := ParseKey(string(text))
	if err != nil {
		return err
	}
	*k = *parsed
	return nil
}

// JSON Marshaler
func (k Key) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// JSON Unmarshaler
func (k *Key) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseKey(s)
	if err != nil {
		return err
	}
	*k = *parsed
	return nil
}

// SQL Valuer
func (k Key) Value() (driver.Value, error) {
	return k.String(), nil
}

// SQL Scanner
func (k *Key) Scan(value any) error {
	switch v := value.(type) {
	case string:
		parsed, err := ParseKey(v)
		if err != nil {
			return err
		}
		*k = *parsed
		return nil
	case []byte:
		parsed, err := ParseKey(string(v))
		if err != nil {
			return err
		}
		*k = *parsed
		return nil
	default:
		return errors.Errorf("cannot scan type %T into Key", value)
	}
}
