package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBufContent(t *testing.T) {
	b := newBuffer(func() {})

	require.Empty(t, b.buf)

	n, err := b.Write([]byte("bacon"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = b.Write([]byte(" lettuce"))
	require.NoError(t, err)
	assert.Equal(t, 8, n)

	n, err = b.Write([]byte(" tomato"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	assert.Equal(t, []byte("bacon lettuce tomato"), b.buf)
}

func TestBufNotifyCallback(t *testing.T) {
	called := false

	b := newBuffer(func() { called = true })
	require.False(t, called)

	_, err := b.Write([]byte("crispy bacon"))
	require.NoError(t, err)
	assert.True(t, called)
}

func TestBufNoNotifyCallback(t *testing.T) {
	called := false

	b := newBuffer(func() { called = true })
	require.False(t, called)

	_, err := b.Write([]byte(""))
	require.NoError(t, err)
	assert.False(t, called, "Notify should be skipped with empty write")
}
