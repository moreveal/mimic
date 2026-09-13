package http3

// RFC 9204 response decoding. The request encoder remains static-only; the
// advertised receive capacity belongs to this connection, never to the pool.
import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/quic-go/qpack"
	"golang.org/x/net/http2/hpack"
)

const qpackStringLimit = 10 << 20

type dynamicQPACK struct {
	mu                                          sync.Mutex
	maxCapacity, capacity, size, total, dropped uint64
	maxBlocked                                  uint64
	blocked                                     map[uint64]bool
	entries                                     []qpack.HeaderField
	changed                                     chan struct{}
	feedback                                    func([]byte) error // serialized under mu; must not acquire mu
}

func newDynamicQPACK(capacity, blocked uint64, feedback func([]byte) error) *dynamicQPACK {
	return &dynamicQPACK{maxCapacity: capacity, maxBlocked: blocked, blocked: make(map[uint64]bool), changed: make(chan struct{}), feedback: feedback}
}

func appendQPACKInt(b []byte, prefix uint8, bits byte, value uint64) []byte {
	mask := uint64(1<<prefix) - 1
	if value < mask {
		return append(b, bits|byte(value))
	}
	b = append(b, bits|byte(mask))
	value -= mask
	for value >= 128 {
		b = append(b, byte(value&127)|128)
		value >>= 7
	}
	return append(b, byte(value))
}

func readQPACKInt(r io.ByteReader, first byte, prefix uint8) (uint64, error) {
	mask := uint64(1<<prefix) - 1
	value := uint64(first) & mask
	if value < mask {
		return value, nil
	}
	for shift := uint(0); shift <= 56; shift += 7 {
		b, err := r.ReadByte()
		if err != nil {
			return 0, io.ErrUnexpectedEOF
		}
		value += uint64(b&127) << shift
		if value > 1<<62-1 {
			return 0, errors.New("QPACK integer overflow")
		}
		if b&128 == 0 {
			return value, nil
		}
	}
	return 0, errors.New("QPACK integer overflow")
}

func readQPACKString(r *bufio.Reader, first byte, prefix uint8, huffman byte, limit uint64) (string, error) {
	n, err := readQPACKInt(r, first, prefix)
	if err != nil {
		return "", err
	}
	if n > limit || n > qpackStringLimit {
		return "", errors.New("QPACK string too large")
	}
	raw := make([]byte, int(n))
	if _, err = io.ReadFull(r, raw); err != nil {
		return "", err
	}
	if first&huffman == 0 {
		return string(raw), nil
	}
	// Huffman expansion is bounded by the shortest code (5 bits). Check the
	// decoded length too, before admitting it to the table / header section.
	value, err := hpack.HuffmanDecodeToString(raw)
	if uint64(len(value)) > limit {
		return "", errors.New("QPACK decoded string too large")
	}
	return value, err
}

func qpackStatic(index uint64) (qpack.HeaderField, error) {
	return qpack.NewDecoder().Decode(appendQPACKInt([]byte{0, 0}, 6, 0xc0, index))()
}

func (d *dynamicQPACK) lookup(index uint64) (qpack.HeaderField, error) {
	if index < d.dropped || index >= d.total {
		return qpack.HeaderField{}, errors.New("QPACK invalid dynamic index")
	}
	return d.entries[index-d.dropped], nil
}

func (d *dynamicQPACK) evict() {
	for d.size > d.capacity && len(d.entries) > 0 {
		h := d.entries[0]
		d.size -= uint64(len(h.Name) + len(h.Value) + 32)
		d.entries[0] = qpack.HeaderField{}
		d.entries = d.entries[1:]
		d.dropped++
	}
}

func (d *dynamicQPACK) insert(h qpack.HeaderField) error {
	size := uint64(len(h.Name) + len(h.Value) + 32)
	if size > d.capacity {
		return errors.New("QPACK entry exceeds capacity")
	}
	d.entries = append(d.entries, h)
	d.size += size
	d.total++
	d.evict()
	// Every insertion is acknowledged before any section acknowledgment can
	// race it; therefore Insert Count Increment can never over-report receipt.
	var err error
	if d.feedback != nil {
		err = d.feedback(appendQPACKInt(nil, 6, 0, 1))
	}
	close(d.changed)
	d.changed = make(chan struct{})
	return err
}

// Each instruction is bounded and committed atomically. A partial instruction
// cannot expose a half-initialized entry to a concurrently unblocked response.
func (d *dynamicQPACK) readEncoder(r io.Reader) error {
	reader := bufio.NewReader(r)
	for {
		first, err := reader.ReadByte()
		if err != nil {
			return err
		}
		var h qpack.HeaderField
		switch {
		case first&0x80 != 0:
			index, err := readQPACKInt(reader, first, 6)
			if err != nil {
				return err
			}
			d.mu.Lock()
			if first&0x40 != 0 {
				h, err = qpackStatic(index)
			} else if index >= d.total {
				err = errors.New("QPACK invalid name reference")
			} else {
				h, err = d.lookup(d.total - index - 1)
			}
			d.mu.Unlock()
			if err != nil {
				return err
			}
			b, err := reader.ReadByte()
			if err != nil {
				return err
			}
			h.Value, err = readQPACKString(reader, b, 7, 0x80, d.maxCapacity)
			if err != nil {
				return err
			}
		case first&0x40 != 0:
			h.Name, err = readQPACKString(reader, first, 5, 0x20, d.maxCapacity)
			if err != nil {
				return err
			}
			b, err := reader.ReadByte()
			if err != nil {
				return err
			}
			h.Value, err = readQPACKString(reader, b, 7, 0x80, d.maxCapacity)
			if err != nil {
				return err
			}
		case first&0x20 != 0:
			capacity, err := readQPACKInt(reader, first, 5)
			if err != nil {
				return err
			}
			if capacity > d.maxCapacity {
				return errors.New("QPACK capacity exceeds SETTINGS")
			}
			d.mu.Lock()
			d.capacity = capacity
			d.evict()
			d.mu.Unlock()
			continue
		default:
			index, err := readQPACKInt(reader, first, 5)
			if err != nil {
				return err
			}
			d.mu.Lock()
			if index >= d.total {
				err = errors.New("QPACK invalid duplicate")
			} else {
				h, err = d.lookup(d.total - index - 1)
			}
			d.mu.Unlock()
			if err != nil {
				return err
			}
		}
		d.mu.Lock()
		err = d.insert(h)
		d.mu.Unlock()
		if err != nil {
			return err
		}
	}
}

func (d *dynamicQPACK) requiredCount(encoded uint64) (uint64, error) {
	if encoded == 0 {
		return 0, nil
	}
	maxEntries := d.maxCapacity / 32
	fullRange := 2 * maxEntries
	if fullRange == 0 || encoded > fullRange {
		return 0, errors.New("QPACK invalid required insert count")
	}
	maxValue := d.total + maxEntries
	required := (maxValue/fullRange)*fullRange + encoded - 1
	if required > maxValue {
		if required <= fullRange {
			return 0, errors.New("QPACK invalid wrapped insert count")
		}
		required -= fullRange
	}
	if required == 0 {
		return 0, errors.New("QPACK zero required insert count")
	}
	return required, nil
}

func qpackDecodeFailure(err error) qpack.DecodeFunc {
	return func() (qpack.HeaderField, error) { return qpack.HeaderField{}, err }
}

// Decode the complete section while holding table ownership, so entries cannot
// be evicted between fields. Wait only for the advertised dependency and release
// the lock while waiting; other request streams remain independent.
func (d *dynamicQPACK) decode(ctx, connection context.Context, streamID uint64, block []byte, limit int) qpack.DecodeFunc {
	headers, err := d.decodeFields(ctx, connection, streamID, block, limit)
	if err != nil {
		return qpackDecodeFailure(err)
	}
	return func() (qpack.HeaderField, error) {
		if len(headers) == 0 {
			return qpack.HeaderField{}, io.EOF
		}
		h := headers[0]
		headers = headers[1:]
		return h, nil
	}
}

func (d *dynamicQPACK) decodeFields(ctx, connection context.Context, streamID uint64, block []byte, limit int) ([]qpack.HeaderField, error) {
	reader := bufio.NewReader(bytes.NewReader(block))
	b, err := reader.ReadByte()
	if err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	encoded, err := readQPACKInt(reader, b, 8)
	if err != nil {
		return nil, err
	}
	b, err = reader.ReadByte()
	if err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	negative := b&128 != 0
	delta, err := readQPACKInt(reader, b, 7)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	required, err := d.requiredCount(encoded)
	if err != nil {
		return nil, err
	}
	base := required + delta
	if negative {
		if delta >= required {
			return nil, errors.New("QPACK negative base")
		}
		base = required - delta - 1
	}
	for required > d.total {
		if !d.blocked[streamID] && uint64(len(d.blocked)) >= d.maxBlocked {
			return nil, errors.New("QPACK blocked stream limit exceeded")
		}
		d.blocked[streamID] = true
		changed := d.changed
		d.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			err = ctx.Err()
		case <-connection.Done():
			err = connection.Err()
		}
		d.mu.Lock()
		delete(d.blocked, streamID)
		if err != nil {
			if d.feedback != nil {
				_ = d.feedback(appendQPACKInt(nil, 6, 0x40, streamID))
			}
			return nil, err
		}
	}
	var fields []qpack.HeaderField
	var largest uint64
	used := 0
	dynamic := func(index uint64) (qpack.HeaderField, error) {
		if index >= required {
			return qpack.HeaderField{}, errors.New("QPACK reference exceeds required count")
		}
		if index+1 > largest {
			largest = index + 1
		}
		return d.lookup(index)
	}
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		var h qpack.HeaderField
		var index uint64
		literalValue := false
		switch {
		case b&0x80 != 0:
			index, err = readQPACKInt(reader, b, 6)
			if err == nil {
				if b&0x40 != 0 {
					h, err = qpackStatic(index)
				} else if index >= base {
					err = errors.New("QPACK invalid relative index")
				} else {
					h, err = dynamic(base - index - 1)
				}
			}
		case b&0xc0 == 0x40:
			literalValue = true
			index, err = readQPACKInt(reader, b, 4)
			if err == nil {
				if b&0x10 != 0 {
					h, err = qpackStatic(index)
				} else if index >= base {
					err = errors.New("QPACK invalid relative name")
				} else {
					h, err = dynamic(base - index - 1)
				}
			}
		case b&0xe0 == 0x20:
			literalValue = true
			h.Name, err = readQPACKString(reader, b, 3, 8, qpackStringLimit)
		case b&0xf0 == 0x10:
			index, err = readQPACKInt(reader, b, 4)
			if err == nil {
				h, err = dynamic(base + index)
			}
		default:
			literalValue = true
			index, err = readQPACKInt(reader, b, 3)
			if err == nil {
				h, err = dynamic(base + index)
			}
		}
		if err != nil {
			return nil, err
		}
		if literalValue {
			b, err = reader.ReadByte()
			if err != nil {
				return nil, io.ErrUnexpectedEOF
			}
			h.Value, err = readQPACKString(reader, b, 7, 128, qpackStringLimit)
			if err != nil {
				return nil, err
			}
		}
		used += len(h.Name) + len(h.Value) + 32
		if used > qpackStringLimit || (limit >= 0 && used > limit) {
			return nil, errHeaderTooLarge
		}
		fields = append(fields, h)
	}
	if largest != required {
		return nil, fmt.Errorf("QPACK required count %d does not match references %d", required, largest)
	}
	if required > 0 && d.feedback != nil {
		if err := d.feedback(appendQPACKInt(nil, 7, 128, streamID)); err != nil {
			return nil, err
		}
	}
	return fields, nil
}

// The local request encoder never inserts entries. The peer can cancel a
// request stream, but cannot acknowledge dynamic sections or received inserts.
func readStaticEncoderFeedback(r io.Reader) error {
	reader := bufio.NewReader(r)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		prefix := uint8(6)
		if b&128 != 0 {
			prefix = 7
		}
		_, err = readQPACKInt(reader, b, prefix)
		if err != nil {
			return err
		}
		if b&0xc0 != 0x40 {
			return errors.New("QPACK acknowledgment for static-only encoder")
		}
	}
}
