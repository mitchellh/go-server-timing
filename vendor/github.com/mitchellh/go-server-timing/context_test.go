package servertiming

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) {
	h := new(Header)
	ctx := NewContext(context.Background(), h)
	h2 := FromContext(ctx)
	if h != h2 {
		t.Fatal("should have stored value")
	}
}

func TestContext_notSet(t *testing.T) {
	h := FromContext(context.Background())
	if h != nil {
		t.Fatal("h should be nil")
	}
}
