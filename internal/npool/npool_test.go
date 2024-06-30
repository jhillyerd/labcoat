package npool_test

import (
	"context"
	"testing"
	"time"

	"github.com/jhillyerd/labcoat/internal/npool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	ctx := context.Background()
	np := npool.New("mylabel", 2)

	w1, err := np.Get(ctx)
	require.NoError(t, err)
	got := w1.String()
	want := "mylabel:1"
	assert.Equal(t, want, got)

	w2, err := np.Get(ctx)
	require.NoError(t, err)
	got = w2.String()
	want = "mylabel:2"
	assert.Equal(t, want, got)
}

func TestDone(t *testing.T) {
	ctx := context.Background()
	np := npool.New("test", 1)

	w1, err := np.Get(ctx)
	require.NoError(t, err)
	id1 := w1.String()
	w1.Done()

	w2, err := np.Get(ctx)
	require.NoError(t, err)
	id2 := w2.String()
	w2.Done()

	assert.Equal(t, id1, id2)
}

func TestBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	np := npool.New("test", 1)

	w1, err := np.Get(ctx)
	require.NoError(t, err)
	assert.NotNil(t, w1)

	w2, err := np.Get(ctx)
	assert.Nil(t, w2, "Get() should have timed out")
	assert.Error(t, err, "Get() should have errored")
}
