package unique

import (
	"fmt"
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestMake_Deduplication(t *testing.T) {
	Clear()

	a := Make("hello")
	b := Make("hell" + "o") // distinct allocation, same content

	require.Equal(t, a.Value(), b.Value())
	require.Equal(t, a, b, "handles for the same string must be equal")
	require.Equal(t, 1, Size())
}

func TestMake_DistinctValues(t *testing.T) {
	Clear()

	a := Make("foo")
	b := Make("bar")

	require.NotEqual(t, a.Value(), b.Value())
	require.NotEqual(t, a, b)
	require.Equal(t, 2, Size())
}

func TestMake_UnsafeStringInput(t *testing.T) {
	Clear()

	// Simulate the protobuf unmarshal pattern: unsafe.String over a byte buffer.
	buf := []byte("ephemeral_value")
	unsafeStr := unsafe.String(unsafe.SliceData(buf), len(buf))
	h := Make(unsafeStr)
	require.Equal(t, "ephemeral_value", h.Value())

	// Mutate the original buffer to prove the interned string is detached.
	buf[0] = 'X'
	require.Equal(t, "ephemeral_value", h.Value(),
		"interned string must not alias the input buffer")
}

func TestMake_Concurrent(t *testing.T) {
	Clear()

	const goroutines = 32
	const keysPerGoroutine = 1000

	keys := make([]string, 100)
	for i := range keys {
		keys[i] = fmt.Sprintf("label_%03d", i)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	results := make([][]Handle, goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			local := make([]Handle, 0, keysPerGoroutine)
			for i := 0; i < keysPerGoroutine; i++ {
				local = append(local, Make(keys[i%len(keys)]))
			}
			results[g] = local
		}()
	}
	wg.Wait()

	require.Equal(t, 100, Size(), "should have exactly 100 unique interned strings")

	// All goroutines should have gotten the same Handle for the same key.
	for i := 0; i < len(keys); i++ {
		canonical := results[0][i]
		for g := 1; g < goroutines; g++ {
			require.Equal(t, canonical, results[g][i],
				"all goroutines must get the same handle for key %q", keys[i])
		}
	}
}

func TestClear(t *testing.T) {
	Clear()

	Make("a")
	Make("b")
	require.Equal(t, 2, Size())

	Clear()
	require.Equal(t, 0, Size())

	Make("c")
	require.Equal(t, 1, Size())
}
