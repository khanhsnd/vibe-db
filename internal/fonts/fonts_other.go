//go:build !windows

package fonts

// Families answers nothing off Windows, where the font list is not the registry's to give. The
// picker then offers the families the stylesheet ships with.
func Families() []string { return nil }
