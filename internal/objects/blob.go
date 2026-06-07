package objects

// Blob is the content of a file.
type Blob struct {
	Data []byte
}

// Type implements Object.
func (b *Blob) Type() Type { return TypeBlob }

// Body implements Object.
func (b *Blob) Body() []byte { return b.Data }
