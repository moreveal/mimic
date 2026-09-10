package network

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/bogdanfinn/quic-go-utls/quicvarint"
	"golang.org/x/crypto/cryptobyte"
)

type wireHello struct {
	ciphers    []uint16
	extensions map[uint16][]byte
	order      []uint16
	sessionID  []byte
}

func parseWireHello(t *testing.T, raw []byte) wireHello {
	t.Helper()
	h := wireHello{extensions: map[uint16][]byte{}}
	s := cryptobyte.String(raw)
	var kind uint8
	var body cryptobyte.String
	if !s.ReadUint8(&kind) || kind != 1 || !s.ReadUint24LengthPrefixed(&body) || !s.Empty() {
		t.Fatal("invalid ClientHello envelope")
	}
	var sid, ciphers, compression, extensions cryptobyte.String
	if !body.Skip(34) || !body.ReadUint8LengthPrefixed(&sid) || !body.ReadUint16LengthPrefixed(&ciphers) || !body.ReadUint8LengthPrefixed(&compression) || !body.ReadUint16LengthPrefixed(&extensions) || !body.Empty() {
		t.Fatal("invalid ClientHello vectors")
	}
	h.sessionID = slices.Clone(sid)
	for !ciphers.Empty() {
		var n uint16
		if !ciphers.ReadUint16(&n) {
			t.Fatal("invalid cipher list")
		}
		h.ciphers = append(h.ciphers, n)
	}
	for !extensions.Empty() {
		var id uint16
		var data cryptobyte.String
		if !extensions.ReadUint16(&id) || !extensions.ReadUint16LengthPrefixed(&data) {
			t.Fatal("invalid extension")
		}
		if _, ok := h.extensions[id]; ok {
			t.Fatalf("duplicate extension %x", id)
		}
		h.extensions[id] = slices.Clone(data)
		h.order = append(h.order, id)
	}
	return h
}
func isGREASE(n uint16) bool { return n&0x0f0f == 0x0a0a && n>>8 == n&255 }
func wireJA4(t *testing.T, h wireHello, transport byte) string {
	t.Helper()
	var ciphers, extensions, signatures []string
	for _, n := range h.ciphers {
		if !isGREASE(n) {
			ciphers = append(ciphers, fmt.Sprintf("%04x", n))
		}
	}
	slices.Sort(ciphers)
	count := 0
	for n := range h.extensions {
		if isGREASE(n) {
			continue
		}
		count++
		if n != 0 && n != 16 {
			extensions = append(extensions, fmt.Sprintf("%04x", n))
		}
	}
	slices.Sort(extensions)
	sig := cryptobyte.String(h.extensions[13])
	var list cryptobyte.String
	if !sig.ReadUint16LengthPrefixed(&list) {
		t.Fatal("missing signature list")
	}
	for !list.Empty() {
		var n uint16
		if !list.ReadUint16(&n) {
			t.Fatal("invalid signature")
		}
		if !isGREASE(n) {
			signatures = append(signatures, fmt.Sprintf("%04x", n))
		}
	}
	sni := 'i'
	if _, ok := h.extensions[0]; ok {
		sni = 'd'
	}
	alpn := cryptobyte.String(h.extensions[16])
	var protocols, first cryptobyte.String
	if !alpn.ReadUint16LengthPrefixed(&protocols) || !protocols.ReadUint8LengthPrefixed(&first) || len(first) == 0 {
		t.Fatal("missing ALPN")
	}
	hash := func(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:6]) }
	// This helper is intentionally scoped to the TLS 1.3 browser fixtures.
	return fmt.Sprintf("%c13%c%02d%02d%c%c_%s_%s", transport, sni, len(ciphers), count, first[0], first[len(first)-1], hash(strings.Join(ciphers, ",")), hash(strings.Join(extensions, ",")+"_"+strings.Join(signatures, ",")))
}

// Record the actual UDP input to the server. Only Initials are retained; the
// public QUIC v1 initial keys suffice, so no application secrets are recorded.
type initialCapture struct {
	net.PacketConn
	mu      sync.Mutex
	packets [][]byte
	peers   []string
}

func (c *initialCapture) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, e := c.PacketConn.ReadFrom(b)
	if n > 0 && b[0]&0xf0 == 0xc0 {
		c.mu.Lock()
		c.packets = append(c.packets, slices.Clone(b[:n]))
		c.peers = append(c.peers, a.String())
		c.mu.Unlock()
	}
	return n, a, e
}
func (c *initialCapture) hellos(t *testing.T) []wireHello {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := map[string][]byte{}
	chunks := map[string]map[uint64]byte{}
	var order []string
	for i, b := range c.packets {
		peer := c.peers[i]
		if _, ok := keys[peer]; !ok {
			keys[peer] = slices.Clone(b[6 : 6+int(b[5])])
			chunks[peer] = map[uint64]byte{}
			order = append(order, peer)
		}
		data, err := decryptInitial(b, keys[peer])
		if err != nil {
			t.Fatal(err)
		}
		r := strings.NewReader(string(data))
		for r.Len() > 0 {
			typ, err := quicvarint.Read(r)
			if err != nil {
				t.Fatal(err)
			}
			if typ == 0 || typ == 1 {
				continue
			}
			if typ == 6 {
				off, err := quicvarint.Read(r)
				if err != nil {
					t.Fatal(err)
				}
				n, err := quicvarint.Read(r)
				if err != nil {
					t.Fatal(err)
				}
				payload := make([]byte, n)
				if _, err = io.ReadFull(r, payload); err != nil {
					t.Fatal(err)
				}
				for j, v := range payload {
					chunks[peer][off+uint64(j)] = v
				}
				continue
			}
			// Later Initials may ACK the server. The first ClientHello is already captured.
			break
		}
	}
	var result []wireHello
	for _, peer := range order {
		data := chunks[peer]
		if len(data) < 4 {
			t.Fatal("missing ClientHello")
		}
		n := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
		b := make([]byte, n+4)
		for i := range b {
			v, ok := data[uint64(i)]
			if !ok {
				t.Fatalf("ClientHello hole at %d", i)
			}
			b[i] = v
		}
		result = append(result, parseWireHello(t, b))
	}
	return result
}
func decryptInitial(packet, dcid []byte) ([]byte, error) {
	b := slices.Clone(packet)
	if len(b) < 7 || binary.BigEndian.Uint32(b[1:5]) != 1 {
		return nil, fmt.Errorf("not QUIC v1")
	}
	p := 6 + int(b[5])
	p += 1 + int(b[p])
	token, n, e := quicvarint.Parse(b[p:])
	if e != nil {
		return nil, e
	}
	p += n + int(token)
	length, n, e := quicvarint.Parse(b[p:])
	if e != nil {
		return nil, e
	}
	p += n
	mac := hmac.New(sha256.New, []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a})
	mac.Write(dcid)
	secret := mac.Sum(nil)
	expand := func(secret []byte, label string, n int) []byte {
		label = "tls13 " + label
		info := []byte{byte(n >> 8), byte(n), byte(len(label))}
		info = append(info, label...)
		info = append(info, 0, 1)
		mac := hmac.New(sha256.New, secret)
		mac.Write(info)
		return mac.Sum(nil)[:n]
	}
	secret = expand(secret, "client in", 32)
	hp, _ := aes.NewCipher(expand(secret, "quic hp", 16))
	mask := make([]byte, 16)
	hp.Encrypt(mask, b[p+4:p+20])
	b[0] ^= mask[0] & 15
	pnlen := int(b[0]&3) + 1
	var pn uint64
	for i := 0; i < pnlen; i++ {
		b[p+i] ^= mask[i+1]
		pn = pn<<8 | uint64(b[p+i])
	}
	iv := expand(secret, "quic iv", 12)
	binary.BigEndian.PutUint64(iv[4:], binary.BigEndian.Uint64(iv[4:])^pn)
	block, _ := aes.NewCipher(expand(secret, "quic key", 16))
	aead, _ := cipher.NewGCM(block)
	return aead.Open(nil, iv, b[p+pnlen:p+int(length)], b[:p+pnlen])
}
