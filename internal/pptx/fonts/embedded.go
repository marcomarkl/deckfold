package fonts

import (
	"bytes"
	"fmt"

	tfont "github.com/tdewolff/font"
)

// Deobfuscate turns a font stored in a presentation (ppt/fonts/*.fntdata) into a plain SFNT file.
// PowerPoint stores embedded fonts as Embedded OpenType; tdewolff/font reads EOT, WOFF and WOFF2.
// The exact format PowerPoint 16 writes still has to be checked against a real file (spike S7),
// so an unknown format is reported instead of guessed.
func Deobfuscate(data []byte, part string) ([]byte, error) {
	if bytes.HasPrefix(data, []byte{0, 1, 0, 0}) || bytes.HasPrefix(data, []byte("OTTO")) || bytes.HasPrefix(data, []byte("true")) {
		return data, nil
	}
	out, err := tfont.ToSFNT(data)
	if err != nil {
		return nil, fmt.Errorf("%s: unknown font format (%v)", part, err)
	}
	return out, nil
}
