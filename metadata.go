package hyperfeed

// Metadata represents a normalized bag of properties for non-standard fields.
type Metadata map[string]Property

type Property struct {
	Namespace string
	Name      string
	Value     string
	RawValue  []byte
}
