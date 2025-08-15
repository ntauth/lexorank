package lexorank

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"fmt"
	"math/big"
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
	// Default alphabet for encoding positions
	defaultAlphabet = []byte("0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz")

	// Default base for our character set (75 characters)
	defaultBase = big.NewInt(75)
)

func TopOf(b uint8, config *Config) Key {
	rank := make([]byte, config.MaxRankLength)
	for i := range rank {
		rank[i] = Maximum
	}

	raw := append([]byte{byte(b + '0'), '|'}, rank...)

	return Key{
		raw:    raw,
		rank:   rank,
		bucket: b,
	}
}

func MiddleOf(b uint8, config *Config) Key {
	rank := make([]byte, config.MaxRankLength)
	for i := range rank {
		rank[i] = Midpoint
	}

	raw := append([]byte{byte(b + '0'), '|'}, rank...)

	return Key{
		raw:    raw,
		rank:   rank,
		bucket: b,
	}
}

func BottomOf(b uint8, config *Config) Key {
	rank := make([]byte, config.MaxRankLength)
	for i := range rank {
		rank[i] = Minimum
	}

	raw := append([]byte{byte(b + '0'), '|'}, rank...)

	return Key{
		raw:    raw,
		rank:   rank,
		bucket: b,
	}
}

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

// ToBigInt converts the key's rank to a big.Int representation
func (k Key) ToBigInt() *big.Int {
	return decodeBase75ToBigInt(k.rank)
}

// FromBigInt creates a new key from a big.Int value
func FromBigInt(bucket uint8, value *big.Int) (*Key, error) {
	rank := encodeBigIntToBase75(value)
	return parseRaw(bucket, rank)
}

// Add returns a new key that is the result of adding the given distance
func (k Key) Add(distance *big.Int) (*Key, error) {
	value := k.ToBigInt()
	result := new(big.Int).Add(value, distance)
	return FromBigInt(k.bucket, result)
}

// Subtract returns a new key that is the result of subtracting the given distance
func (k Key) Subtract(distance *big.Int) (*Key, error) {
	value := k.ToBigInt()
	result := new(big.Int).Sub(value, distance)
	if result.Sign() < 0 {
		return nil, ErrOutOfBounds
	}
	return FromBigInt(k.bucket, result)
}

// Multiply returns a new key that is the result of multiplying by the given factor
func (k Key) Multiply(factor *big.Int) (*Key, error) {
	value := k.ToBigInt()
	result := new(big.Int).Mul(value, factor)
	return FromBigInt(k.bucket, result)
}

// Divide returns a new key that is the result of dividing by the given divisor
func (k Key) Divide(divisor *big.Int) (*Key, error) {
	if divisor.Sign() == 0 {
		return nil, fmt.Errorf("division by zero")
	}
	value := k.ToBigInt()
	result := new(big.Int).Div(value, divisor)
	return FromBigInt(k.bucket, result)
}

// Distance returns the distance between this key and another key
func (k Key) Distance(other Key) *big.Int {
	thisValue := k.ToBigInt()
	otherValue := other.ToBigInt()
	return new(big.Int).Sub(otherValue, thisValue)
}

type Keys []Key

type Rank []byte

func ParseKey(s string) (*Key, error) {
	if len(s) < 3 {
		return nil, fmt.Errorf("invalid key length: %d (minimum 3)", len(s))
	}

	bucket, err := strconv.Atoi(string(s[0]))
	if err != nil {
		return nil, err
	}

	rank := []byte(s[2:])

	return parseRaw(uint8(bucket), rank)
}

func parseRaw(bucket uint8, rank []byte) (*Key, error) {
	if len(rank) == 0 {
		return nil, fmt.Errorf("rank cannot be empty")
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
func KeyAt(bucket uint8, f float64, config *Config) (Key, error) {
	bucketChar := byte(bucket + 48)

	base := float64(len(defaultAlphabet)) // 75
	key := make([]byte, 0, config.MaxRankLength)

	for i := 0; i < config.MaxRankLength; i++ {
		f *= base
		index := int(f)
		if index >= len(defaultAlphabet) {
			index = len(defaultAlphabet) - 1
		}
		key = append(key, defaultAlphabet[index])
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

// Between returns a new key between two keys.
func Between(lhs, rhs Key, config *Config) (*Key, error) {
	// Ensure both keys are in the same bucket
	if lhs.bucket != rhs.bucket {
		return nil, fmt.Errorf("keys must be in the same bucket")
	}

	// Parse the rank digits in base-B (75)
	sa := suffixDigits(lhs.rank)
	sb := suffixDigits(rhs.rank)

	// Determine the minimum length to work with
	L := max(len(sa), len(sb), 1) // At least 1 digit

	// Convert to big.Int in base-B and scale to same length
	na := scaleUpTo(toBigIntBaseB(sa), L)
	nb := scaleUpTo(toBigIntBaseB(sb), L)

	// Ensure proper ordering
	if na.Cmp(nb) >= 0 {
		return nil, fmt.Errorf("left key must be less than right key")
	}

	// Find the mathematical midpoint
	for {
		// Calculate midpoint: floor((na + nb) / 2)
		mid := new(big.Int).Add(na, nb)
		mid.Rsh(mid, 1) // Right shift by 1 = divide by 2

		// Check if this midpoint is strictly between na and nb
		if mid.Cmp(na) > 0 && mid.Cmp(nb) < 0 {
			// We found a valid midpoint, encode it back to base-B
			return makeKey(lhs.bucket, encodeBaseB(mid, L)), nil
		}

		// No integer strictly between at this precision, add one digit
		if config.MaxRankLength > 0 && L >= config.MaxRankLength {
			return nil, ErrRebalanceRequired
		}

		// Scale up by base (75) and try again
		L++
		na.Mul(na, defaultBase)
		nb.Mul(nb, defaultBase)
	}
}

func (k Key) After(distance int64) (*Key, error) {
	step := big.NewInt(distance)
	return k.Add(step)
}

func (k Key) Before(distance int64) (*Key, error) {
	step := big.NewInt(distance)
	return k.Subtract(step)
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
		pos := int64(bytes.IndexByte(defaultAlphabet, c))
		if pos == -1 {
			return 0, errors.New("invalid character in rank")
		}
		index = index*int64(len(defaultAlphabet)) + pos
	}
	return index, nil
}

func encodeBase75(val int64) []byte {
	if val == 0 {
		return []byte{defaultAlphabet[0]}
	}

	var out []byte
	for val > 0 {
		rem := val % int64(len(defaultAlphabet))
		out = append([]byte{defaultAlphabet[rem]}, out...)
		val = val / int64(len(defaultAlphabet))
	}

	return out
}

// decodeBase75ToBigInt converts a base75 rank string to a big.Int
func decodeBase75ToBigInt(rank []byte) *big.Int {
	base := big.NewInt(int64(len(defaultAlphabet)))
	result := big.NewInt(0)

	for _, c := range rank {
		pos := int64(bytes.IndexByte(defaultAlphabet, c))
		if pos == -1 {
			return big.NewInt(0) // Invalid character, return 0
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(pos))
	}

	return result
}

// encodeBigIntToBase75 converts a big.Int to a base75 rank string
func encodeBigIntToBase75(val *big.Int) []byte {
	if val.Sign() == 0 {
		return []byte{defaultAlphabet[0]}
	}

	base := big.NewInt(int64(len(defaultAlphabet)))
	zero := big.NewInt(0)

	var out []byte
	temp := new(big.Int).Set(val)

	for temp.Cmp(zero) > 0 {
		rem := new(big.Int)
		temp.DivMod(temp, base, rem)
		remInt := int(rem.Int64())
		if remInt >= len(defaultAlphabet) {
			remInt = len(defaultAlphabet) - 1
		}
		out = append([]byte{defaultAlphabet[remInt]}, out...)
	}

	return out
}

// suffixDigits converts a rank string to a slice of digit indices in base-B
func suffixDigits(rank []byte) []int {
	digits := make([]int, len(rank))
	for i, char := range rank {
		digits[i] = bytes.IndexByte(defaultAlphabet, char)
		if digits[i] == -1 {
			digits[i] = 0 // Default to 0 for invalid characters
		}
	}
	return digits
}

// toBigIntBaseB converts a slice of base-B digits to a big.Int
func toBigIntBaseB(digits []int) *big.Int {
	result := big.NewInt(0)
	base := big.NewInt(75)

	for _, digit := range digits {
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(digit)))
	}

	return result
}

// scaleUpTo scales a big.Int to a specific length by multiplying by base^(targetLength - currentLength)
func scaleUpTo(val *big.Int, targetLength int) *big.Int {
	currentLength := len(encodeBigIntToBase75(val))
	if currentLength >= targetLength {
		return new(big.Int).Set(val)
	}

	scaleFactor := new(big.Int).Exp(defaultBase, big.NewInt(int64(targetLength-currentLength)), nil)
	return new(big.Int).Mul(val, scaleFactor)
}

// encodeBaseB encodes a big.Int to a base-B string of specified length
func encodeBaseB(val *big.Int, length int) []byte {
	base := big.NewInt(75)

	var out []byte
	temp := new(big.Int).Set(val)

	for range length {
		rem := new(big.Int)
		temp.DivMod(temp, base, rem)
		remInt := int(rem.Int64())
		if remInt >= len(defaultAlphabet) {
			remInt = len(defaultAlphabet) - 1
		}
		out = append([]byte{defaultAlphabet[remInt]}, out...)
	}

	return out
}

// makeKey creates a new Key from bucket and rank
func makeKey(bucket uint8, rank []byte) *Key {
	raw := append([]byte{byte(bucket + '0'), '|'}, rank...)
	return &Key{
		raw:    raw,
		rank:   rank,
		bucket: bucket,
	}
}

// SmartAppend generates a new key for appending using the specified strategy
func SmartAppend(last Key, config *Config) (*Key, error) {
	switch config.AppendStrategy {
	case AppendStrategyDefault:
		return Between(last, TopOf(last.bucket, config), config)
	case AppendStrategyStep:
		step := big.NewInt(config.StepSize)
		return last.Add(step)
	default:
		return Between(last, TopOf(last.bucket, config), config)
	}
}

// SmartPrepend generates a new key for prepending using the specified strategy
func SmartPrepend(first Key, config *Config) (*Key, error) {
	switch config.AppendStrategy {
	case AppendStrategyDefault:
		return Between(BottomOf(first.bucket, config), first, config)
	case AppendStrategyStep:
		step := big.NewInt(config.StepSize)
		return first.Subtract(step)
	default:
		return Between(BottomOf(first.bucket, config), first, config)
	}
}

func Random(config *Config) (Key, error) {
	f := rand.Float64()
	return KeyAt(0, f, config)
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
