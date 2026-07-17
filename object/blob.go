package object

// Blob 只保存文件内容，不保存文件名。
type Blob struct {
	Content []byte
}

func NewBlob(content []byte) *Blob {
	data := make([]byte, len(content))
	copy(data, content)
	return &Blob{Content: data}
}

func (b *Blob) Type() string {
	return "blob"
}

func (b *Blob) Payload() []byte {
	data := make([]byte, len(b.Content))
	copy(data, b.Content)
	return data
}

func (b *Blob) Size() int {
	return len(b.Content)
}

func (b *Blob) Raw() []byte {
	return RawObject(b)
}

func (b *Blob) HashString() string {
	return HashObject(b)
}

func (b *Blob) Write() (string, error) {
	return WriteObject(b)
}

func BlobFromStored(stored *StoredObject) (*Blob, error) {
	if stored.ObjectType != "blob" {
		return nil, ErrUnexpectedType("blob", stored.ObjectType)
	}
	return NewBlob(stored.Payload), nil
}

func ReadBlob(hash string) (*Blob, error) {
	stored, err := ReadObject(hash)
	if err != nil {
		return nil, err
	}
	return BlobFromStored(stored)
}
