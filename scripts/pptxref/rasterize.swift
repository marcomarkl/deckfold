// rasterize.swift <deck.pdf> <out folder> <width> <height>
// Renders every page of a PDF exported by PowerPoint into <out>/<n>.png at the given pixel size
// (the slide size in CSS pixels), on a white background. Used by pptxref.sh, because PowerPoint's
// own PNG export reports success but writes no files (see docs/spikes/S4-powerpoint-reference.md).
import AppKit
import PDFKit

let args = CommandLine.arguments
guard args.count == 5, let doc = PDFDocument(url: URL(fileURLWithPath: args[1])),
      let width = Int(args[3]), let height = Int(args[4]) else {
    FileHandle.standardError.write("usage: rasterize.swift <deck.pdf> <out folder> <width> <height>\n".data(using: .utf8)!)
    exit(2)
}
let out = URL(fileURLWithPath: args[2])
let space = CGColorSpace(name: CGColorSpace.sRGB)!
for i in 0..<doc.pageCount {
    guard let page = doc.page(at: i) else { continue }
    let box = page.bounds(for: .mediaBox)
    guard let ctx = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                              space: space, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { exit(1) }
    ctx.setFillColor(CGColor(red: 1, green: 1, blue: 1, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: width, height: height))
    ctx.interpolationQuality = .high
    ctx.scaleBy(x: CGFloat(width) / box.width, y: CGFloat(height) / box.height)
    ctx.translateBy(x: -box.minX, y: -box.minY)
    page.draw(with: .mediaBox, to: ctx)
    guard let img = ctx.makeImage() else { exit(1) }
    let rep = NSBitmapImageRep(cgImage: img)
    guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
    try png.write(to: out.appendingPathComponent("\(i + 1).png"))
}
print("\(doc.pageCount) pages")
