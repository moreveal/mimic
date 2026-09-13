package http3

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/quic-go/qpack"
)

func qhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func encoderBytes(t *testing.T, d *dynamicQPACK, s string) {
	t.Helper()
	if err := d.readEncoder(bytes.NewReader(qhex(t, s))); err != io.EOF {
		t.Fatal(err)
	}
}
func decodeBytes(t *testing.T, d *dynamicQPACK, s string) []qpack.HeaderField {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h, err := d.decodeFields(ctx, ctx, 4, qhex(t, s), 4096)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestDynamicQPACKRFC9204Examples(t *testing.T) {
	var feedback bytes.Buffer
	d := newDynamicQPACK(220, 2, func(b []byte) error { _, err := feedback.Write(b); return err })
	if got := decodeBytes(t, d, "0000510b2f696e6465782e68746d6c"); !reflect.DeepEqual(got, []qpack.HeaderField{{Name: ":path", Value: "/index.html"}}) {
		t.Fatal(got)
	}
	encoderBytes(t, d, "3fbd01c00f7777772e6578616d706c652e636f6dc10c2f73616d706c652f70617468")
	want := []qpack.HeaderField{{Name: ":authority", Value: "www.example.com"}, {Name: ":path", Value: "/sample/path"}}
	if got := decodeBytes(t, d, "03811011"); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if !bytes.Equal(feedback.Bytes(), []byte{1, 1, 0x84}) {
		t.Fatalf("feedback %x", feedback.Bytes())
	}
	encoderBytes(t, d, "4a637573746f6d2d6b65790c637573746f6d2d76616c7565")
	encoderBytes(t, d, "02")
	got := decodeBytes(t, d, "050080c181")
	if got[0] != want[0] || got[1].Value != "/" || got[2].Value != "custom-value" {
		t.Fatal(got)
	}
	encoderBytes(t, d, "810d637573746f6d2d76616c756532")
	if d.size != 215 || d.dropped != 1 || d.total != 5 {
		t.Fatalf("table: size=%d dropped=%d total=%d", d.size, d.dropped, d.total)
	}
	if got := decodeBytes(t, d, "060080"); got[0].Name != "custom-key" || got[0].Value != "custom-value2" {
		t.Fatal(got)
	}
}

func TestDynamicQPACKLiteralRepresentationsAndHuffman(t *testing.T) {
	d := newDynamicQPACK(128, 1, nil)
	// Literal name x and Huffman value www.example.com (RFC 7541 example).
	encoderBytes(t, d, "3f6141788cf1e3c2e5f23a6ba0ab90f4ff")
	for _, s := range []string{"020080", "028010"} {
		if got := decodeBytes(t, d, s); got[0].Value != "www.example.com" {
			t.Fatal(got)
		}
	}
	// Dynamic relative name, post-base name, and literal name field lines.
	for _, s := range []string{"020040036e6577", "028000036e6577"} {
		if got := decodeBytes(t, d, s); got[0].Name != "x" || got[0].Value != "new" {
			t.Fatal(got)
		}
	}
	if got := decodeBytes(t, d, "00002178036e6577"); got[0].Name != "x" || got[0].Value != "new" {
		t.Fatal(got)
	}
}

func waitQPACKBlocked(t *testing.T, d *dynamicQPACK) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		n := len(d.blocked)
		d.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("decoder did not block")
}

func TestDynamicQPACKBlockedStreamsCancelAndIsolation(t *testing.T) {
	feedback := make(chan []byte, 10)
	d := newDynamicQPACK(128, 1, func(b []byte) error { feedback <- b; return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := d.decodeFields(ctx, ctx, 8, []byte{2, 0, 0x80}, 4096); done <- err }()
	waitQPACKBlocked(t, d)
	// Static-only responses still complete on the same connection.
	if got := decodeBytes(t, d, "0000d9"); got[0].Value != "200" {
		t.Fatal(got)
	}
	if _, err := d.decodeFields(ctx, ctx, 12, []byte{2, 0, 0x80}, 4096); err == nil {
		t.Fatal("blocked-stream limit ignored")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation stalled")
	}
	if b := <-feedback; !bytes.Equal(b, []byte{0x48}) {
		t.Fatalf("cancel %x", b)
	}
	d.mu.Lock()
	blocked := len(d.blocked)
	d.mu.Unlock()
	if blocked != 0 {
		t.Fatal("retained canceled stream")
	}
	next := newDynamicQPACK(128, 1, nil)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	go func() {
		h, err := next.decodeFields(ctx2, ctx2, 4, []byte{2, 0, 0x80}, 4096)
		if err == nil && h[0].Value != "y" {
			err = errors.New("wrong entry")
		}
		done <- err
	}()
	waitQPACKBlocked(t, next)
	encoderBytes(t, d, "3f614178017a")
	select {
	case <-done:
		t.Fatal("table leaked between connections")
	case <-time.After(10 * time.Millisecond):
	}
	encoderBytes(t, next, "3f6141780179")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("insert did not unblock")
	}
}

func TestDynamicQPACKWrapEvictionAndInvalidInput(t *testing.T) {
	d := newDynamicQPACK(64, 1, nil)
	encoderBytes(t, d, "3f2141780179")
	for i := 0; i < 20; i++ {
		encoderBytes(t, d, "00")
	}
	// MaxEntries=2, total=21 => encoded count=(21 mod 4)+1=2.
	if got := decodeBytes(t, d, "020080"); got[0].Value != "y" {
		t.Fatal(got)
	}
	for _, s := range []string{"050080", "020081", "02008080ff", "0200", "0080", "0000ff7f"} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		_, err := d.decodeFields(ctx, ctx, 4, qhex(t, s), 4096)
		cancel()
		if err == nil {
			t.Fatalf("accepted invalid block %s", s)
		}
	}
	for _, s := range []string{"3f22", "3f214178ff", "3f2100", "3f21ff7f00"} {
		next := newDynamicQPACK(64, 1, nil)
		if err := next.readEncoder(bytes.NewReader(qhex(t, s))); err == nil || err == io.EOF {
			t.Fatalf("accepted encoder bytes %s: %v", s, err)
		}
	}
}
