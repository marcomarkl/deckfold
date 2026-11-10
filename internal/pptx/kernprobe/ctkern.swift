// The kerning CoreText applies to each text, for kernprobe (see coretext.go).
// usage: swift - <font file> <PostScript name> text...   (the script comes from standard input)
// output: the PostScript name, then one line per text with its plain width and its kerning in em
import CoreText
import Foundation

let args = CommandLine.arguments
guard args.count >= 3, let descs = CTFontManagerCreateFontDescriptorsFromURL(URL(fileURLWithPath: args[1]) as CFURL) as? [CTFontDescriptor] else {
    FileHandle.standardError.write("cannot read the font file\n".data(using: .utf8)!)
    exit(1)
}
var found: CTFont?
for d in descs {
    let f = CTFontCreateWithFontDescriptor(d, 1000, nil)
    if CTFontCopyPostScriptName(f) as String == args[2] { found = f }
}
guard let font = found else {
    FileHandle.standardError.write("no face named \(args[2]) in the file\n".data(using: .utf8)!)
    exit(1)
}
func width(_ s: String, kern: Bool) -> Double {
    var attrs: [NSAttributedString.Key: Any] = [
        NSAttributedString.Key(kCTFontAttributeName as String): font,
        NSAttributedString.Key(kCTLigatureAttributeName as String): 0,
    ]
    if !kern { attrs[NSAttributedString.Key(kCTKernAttributeName as String)] = 0.0 }
    let line = CTLineCreateWithAttributedString(NSAttributedString(string: s, attributes: attrs))
    return CTLineGetTypographicBounds(line, nil, nil, nil) / 1000
}
print(CTFontCopyPostScriptName(font) as String)
for t in args.dropFirst(3) {
    let plain = width(t, kern: false)
    print(String(format: "%.6f %.6f", plain, width(t, kern: true) - plain))
}
