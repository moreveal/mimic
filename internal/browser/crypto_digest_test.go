package browser

import (
	"context"
	"testing"
)

// The SHA-256 value is also captured by the controlled Chrome 152.0.7977.82
// oracle. Keeping the literal here prevents an incorrect external expectation
// from becoming an implementation change.
func TestSubtleCryptoDigestKnownVectors(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(async()=>{
 const hex=async(name,text)=>[...new Uint8Array(await crypto.subtle.digest(name,new TextEncoder().encode(text)))].map(byte=>byte.toString(16).padStart(2,'0')).join('');
 return [await hex('SHA-1','mimic'),await hex('SHA-256','mimic'),await hex('SHA-384',''),await hex('SHA-512','')].join('|');
})()`)
		if err != nil {
			t.Fatal(err)
		}
		const want = "76c54c7e68822e3cc743c8b29445c7890aa8d73d|692e51978c0e4aa1f130e5cb9a536421f7925c8bae0a2291414791b9f86ee000|38b060a751ac96384cd9327eb1b1e36a21fdb71114be07434c0cc7bf63f6e1da274edebfe76f65fbd51ad2f14898b95b|cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
		if value != want {
			t.Fatalf("digest vectors: got %v want %v", value, want)
		}
	})
}
