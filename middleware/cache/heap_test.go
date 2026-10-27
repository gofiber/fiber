package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexedHeapGenerationSurvivesIndexReuse(t *testing.T) {
	t.Parallel()

	h := &indexedHeap{}
	firstIdx := h.put("same-key", 1, 10, "")
	firstGen := h.generation(firstIdx)
	require.True(t, h.matches("same-key", firstIdx, firstGen))
	require.False(t, h.matches("same-key", -1, firstGen))
	require.False(t, h.matches("same-key", len(h.indices), firstGen))

	key, size := h.remove(firstIdx)
	require.Equal(t, "same-key", key)
	require.Equal(t, uint(10), size)
	require.False(t, h.matches("same-key", firstIdx, firstGen))

	secondIdx := h.put("same-key", 2, 20, "")
	secondGen := h.generation(secondIdx)
	require.Equal(t, firstIdx, secondIdx)
	require.NotEqual(t, firstGen, secondGen)
	require.False(t, h.matches("same-key", firstIdx, firstGen))
	require.True(t, h.matches("same-key", secondIdx, secondGen))
	require.False(t, h.matches("other-key", secondIdx, secondGen))
	require.Equal(t, 1, h.Len())
	require.Equal(t, uint(20), h.entries[0].bytes)
}

func TestIndexedHeapOrphansAManifestWithItsLastEntry(t *testing.T) {
	t.Parallel()

	h := &indexedHeap{}
	a := h.put("m|a", 1, 1, "m")
	b := h.put("m|b", 2, 1, "m")
	h.put("plain", 3, 1, "")

	h.remove(a)
	require.True(t, h.tracks("m"))
	require.Nil(t, h.takeOrphans())

	h.remove(b)
	require.False(t, h.tracks("m"))
	h.put("m|b", 4, 1, "m") // a replacement stored under it again
	require.Nil(t, h.takeOrphans())

	h.remove(h.keys["m|b"])
	require.Equal(t, []string{"m"}, h.takeOrphans())
	require.Nil(t, h.takeOrphans())
}
