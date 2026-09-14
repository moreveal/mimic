package engine

// BinaryBuffer asks the runtime to copy bytes into a realm-owned ArrayBuffer.
// The caller keeps ownership of the source slice and may reuse it after the
// synchronous conversion returns. JavaScript writes must never alias the source.
// Unlike ordinary byte slices, this explicit transport value is not a JS array
// or a base64 string. It can be nested in host records and Promise results.
type BinaryBuffer []byte
