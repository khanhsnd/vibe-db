//go:build windows

package fonts

import (
	"maps"
	"slices"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const fontsKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`

// Families lists the font families installed on this machine — the machine-wide hive and the
// reader's own, since a font installed for one user is registered only there. A hive that cannot
// be read contributes nothing rather than failing the call: a short list is still a usable picker.
func Families() []string {
	seen := map[string]bool{}

	for _, hive := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, entry := range names(hive) {
			// A family drawn top to bottom is registered under `@` and is not one a grid can use.
			if family := familyOf(entry); family != "" && !strings.HasPrefix(family, "@") {
				seen[family] = true
			}
		}
	}

	return slices.Sorted(maps.Keys(seen))
}

func names(hive registry.Key) []string {
	key, err := registry.OpenKey(hive, fontsKey, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	values, err := key.ReadValueNames(0)
	if err != nil {
		return nil
	}
	return values
}
