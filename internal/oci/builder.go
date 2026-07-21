package oci

// Builder is a minimal image builder abstraction used by tests.
type Builder struct {
	Image string
}

// NewBuilder constructs a Builder for a named image.
func NewBuilder(image string) *Builder {
	return &Builder{Image: image}
}

// Build returns the full image reference for a given tag.
func (b *Builder) Build(tag string) string {
	if tag == "" {
		tag = "latest"
	}
	return b.Image + ":" + tag
}
