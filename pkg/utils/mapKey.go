package utils

import (
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"time"
)

// MapKey generates a deterministic, fixed-length Redis key suffix.
//
// Output:
//   - 16 hex characters (64-bit FNV-1a)
//
// Supported value types:
//   - string
//   - bool
//   - all int / uint variants
//   - float32 / float64
//   - time.Time
//   - nil
//
// Any unsupported type will cause a panic (intentional).
func MapKey[T any](m map[string]T) string {
	h := fnv.New64a()
	writeMapToHash(h, m)
	return fmt.Sprintf("%016x", h.Sum64())
}

// writeMapToHash writes a canonical representation of the map into the hash.
// Map order is normalized by sorting keys.
func writeMapToHash[T any](h hash.Hash, m map[string]T) {
	if len(m) == 0 {
		return
	}

	// 1. sort keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. write key=value pairs
	for _, k := range keys {
		writeString(h, k)
		writeByte(h, '=')

		v := any(m[k])
		switch x := v.(type) {
		case nil:
			// key=
		case string:
			writeString(h, x)
		case bool:
			if x {
				writeByte(h, '1')
			} else {
				writeByte(h, '0')
			}
		case int:
			writeInt(h, int64(x))
		case int8:
			writeInt(h, int64(x))
		case int16:
			writeInt(h, int64(x))
		case int32:
			writeInt(h, int64(x))
		case int64:
			writeInt(h, x)
		case uint:
			writeUint(h, uint64(x))
		case uint8:
			writeUint(h, uint64(x))
		case uint16:
			writeUint(h, uint64(x))
		case uint32:
			writeUint(h, uint64(x))
		case uint64:
			writeUint(h, x)
		case float32:
			writeFloat(h, float64(x), 32)
		case float64:
			writeFloat(h, x, 64)
		case time.Time:
			if !x.IsZero() {
				// UTC + RFC3339Nano for stability
				writeString(h, x.UTC().Format(time.RFC3339Nano))
			}
		default:
			panic(fmt.Sprintf("MapKey: unsupported value type %T", x))
		}

		writeByte(h, '&')
	}
}

/* =======================
   low-level write helpers
   ======================= */

func writeByte(h hash.Hash, b byte) {
	_, _ = h.Write([]byte{b})
}

func writeString(h hash.Hash, s string) {
	_, _ = h.Write([]byte(s))
}

func writeInt(h hash.Hash, v int64) {
	var buf [20]byte
	n := strconv.AppendInt(buf[:0], v, 10)
	_, _ = h.Write(n)
}

func writeUint(h hash.Hash, v uint64) {
	var buf [20]byte
	n := strconv.AppendUint(buf[:0], v, 10)
	_, _ = h.Write(n)
}

func writeFloat(h hash.Hash, v float64, bits int) {
	var buf [32]byte
	n := strconv.AppendFloat(buf[:0], v, 'g', -1, bits)
	_, _ = h.Write(n)
}
