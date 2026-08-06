package transfer

import (
	"fmt"
	"io"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Encoding is one way of reading a file's bytes as text. The list is closed and the frontend
// mirrors it, exactly as the engine and TLS lists are mirrored: a name that is not here is refused
// rather than guessed at.
//
// It lives here rather than beside the one caller it started with, because every file this app
// opens asks the same question — a script to run, a CSV to import, a workbook to write — and one
// list is what keeps the answer the same in all three places.
type Encoding struct {
	Name  string
	Label string
	// Charset is what this encoding is called by a *file that names its own* — an XML declaration,
	// an HTML meta tag. It is the IANA name rather than ours, and the two UTF-16 entries share one
	// because a byte order mark is what tells those apart and the declaration cannot.
	Charset string
	Codec   encoding.Encoding
}

// The UTF-8 entry is UTF8BOM rather than UTF8, and both halves of that are wanted. Its **decoder**
// strips a leading byte order mark, and a file exported by a Windows tool nearly always carries one
// — left in place it would become part of the first statement's first keyword, or of the first
// column's own name. Its **encoder** writes one, which is what makes Excel open a UTF-8 CSV as UTF-8
// instead of as the machine's own code page.
var Encodings = []Encoding{
	{"utf-8", "UTF-8", "UTF-8", unicode.UTF8BOM},
	{"utf-16le", "UTF-16 LE", "UTF-16", unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)},
	{"utf-16be", "UTF-16 BE", "UTF-16", unicode.UTF16(unicode.BigEndian, unicode.UseBOM)},
	{"windows-1252", "Windows-1252 (Western European)", "windows-1252", charmap.Windows1252},
	{"windows-1258", "Windows-1258 (Vietnamese)", "windows-1258", charmap.Windows1258},
	{"iso-8859-1", "ISO-8859-1 (Latin 1)", "ISO-8859-1", charmap.ISO8859_1},
}

// DefaultEncoding is what an empty name means, and what every box offering the list starts on.
const DefaultEncoding = "utf-8"

func EncodingNamed(name string) (Encoding, error) {
	if name == "" {
		name = DefaultEncoding
	}
	for _, candidate := range Encodings {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return Encoding{}, domain.ValidationError{Field: "encoding", Message: "is not supported"}
}

// Decode reads a whole file's bytes as the named encoding. A file whose bytes are not that encoding
// fails the same way one that could not be opened does, and for the same reason: the reader picked
// the wrong file or the wrong encoding, and both are answered in the dialog they picked them in.
func Decode(raw []byte, name string) (string, error) {
	chosen, err := EncodingNamed(name)
	if err != nil {
		return "", err
	}

	decoded, err := chosen.Codec.NewDecoder().Bytes(raw)
	if err != nil {
		return "", domain.FileError{Err: fmt.Errorf("not %s", chosen.Label)}
	}
	return string(decoded), nil
}

func decodeReader(in io.Reader, name string) (io.Reader, error) {
	chosen, err := EncodingNamed(name)
	if err != nil {
		return nil, err
	}
	return chosen.Codec.NewDecoder().Reader(in), nil
}

func encodeWriter(out io.Writer, name string) (io.Writer, error) {
	chosen, err := EncodingNamed(name)
	if err != nil {
		return nil, err
	}
	return chosen.Codec.NewEncoder().Writer(out), nil
}
