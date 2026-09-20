package layoutblitz

// ComputedStyle uses the same native CSSOM serializer as batch publication.
// Logical mapping belongs in that serializer so scalar and batch reads agree.
func (o *Owner) ComputedStyle(id uint64, name string) (string, error) {
	return o.Style(id, name)
}
