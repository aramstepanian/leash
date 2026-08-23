#!/usr/bin/env swift
import AppKit
import Foundation

// Clip-and-strap mark → App Icon + menu bar template images.
// Run from repo root: swift macos/render-assets.swift

let paper = NSColor(srgbRed: 0.957, green: 0.945, blue: 0.925, alpha: 1)
let ink = NSColor(srgbRed: 0.102, green: 0.098, blue: 0.086, alpha: 1)

struct Mark {
    var ring: CGRect
    var strap: CGRect
    var line: CGFloat
}

func layout(canvas: CGSize) -> Mark {
    let s = min(canvas.width, canvas.height)
    let d = s * 0.72
    let line = max(1.6, d * 0.16)
    let strapH = max(line * 1.15, d * 0.24)
    let strapW = d * 0.36
    let overlap = line * 0.50
    let groupW = d - overlap + strapW
    let x0 = (canvas.width - groupW) / 2
    let y0 = (canvas.height - d) / 2
    return Mark(
        ring: CGRect(x: x0, y: y0, width: d, height: d),
        strap: CGRect(
            x: x0 + d - overlap,
            y: y0 + (d - strapH) / 2,
            width: strapW,
            height: strapH
        ),
        line: line
    )
}

func strokeRing(rect: CGRect, line: CGFloat, color: NSColor) {
    let path = NSBezierPath(ovalIn: rect.insetBy(dx: line / 2, dy: line / 2))
    color.setStroke()
    path.lineWidth = line
    path.lineCapStyle = .round
    path.lineJoinStyle = .round
    path.stroke()
}

func fillCircle(_ rect: CGRect, color: NSColor) {
    let path = NSBezierPath(ovalIn: rect)
    color.setFill()
    path.fill()
}

func fillStrap(_ rect: CGRect, color: NSColor) {
    let path = NSBezierPath(roundedRect: rect, xRadius: rect.height / 2, yRadius: rect.height / 2)
    color.setFill()
    path.fill()
}

func drawMark(canvas: CGSize, color: NSColor, filled: Bool, compact: Bool = false) {
    let mark = layout(canvas: canvas)
    if filled || compact {
        fillCircle(mark.ring, color: color)
    } else {
        strokeRing(rect: mark.ring, line: mark.line, color: color)
    }
    fillStrap(mark.strap, color: color)
}

func makeRep(pixels: Int, opaque: Bool, draw: (CGSize) -> Void) -> NSBitmapImageRep {
    let colorSpace = CGColorSpace(name: CGColorSpace.sRGB)!
    let bitmapInfo = CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue
    guard let cg = CGContext(
        data: nil,
        width: pixels,
        height: pixels,
        bitsPerComponent: 8,
        bytesPerRow: 0,
        space: colorSpace,
        bitmapInfo: bitmapInfo
    ) else {
        fatalError("cg context")
    }
    cg.clear(CGRect(x: 0, y: 0, width: pixels, height: pixels))
    let ns = NSGraphicsContext(cgContext: cg, flipped: false)
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = ns
    ns.shouldAntialias = true
    ns.imageInterpolation = .high
    if opaque {
        paper.setFill()
        NSBezierPath(rect: NSRect(origin: .zero, size: NSSize(width: pixels, height: pixels))).fill()
    }
    draw(CGSize(width: pixels, height: pixels))
    NSGraphicsContext.restoreGraphicsState()
    guard let image = cg.makeImage() else { fatalError("cg image") }
    return NSBitmapImageRep(cgImage: image)
}

func writePNG(_ rep: NSBitmapImageRep, to url: URL) throws {
    guard let png = rep.representation(using: .png, properties: [:]) else {
        throw NSError(domain: "render-assets", code: 1, userInfo: [NSLocalizedDescriptionKey: "png encode failed \(url.lastPathComponent)"])
    }
    try png.write(to: url)
}

func writeJSON(_ object: Any, to url: URL) throws {
    let data = try JSONSerialization.data(withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    try data.write(to: url)
}

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
    .appendingPathComponent("macos/Leash/Assets.xcassets")
let iconDir = root.appendingPathComponent("AppIcon.appiconset")
let outlineDir = root.appendingPathComponent("LeashMenu.imageset")
let filledDir = root.appendingPathComponent("LeashMenuFilled.imageset")

for dir in [iconDir, outlineDir, filledDir] {
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
}

let catalog: [String: Any] = [
    "info": ["author": "xcode", "version": 1]
]
try writeJSON(catalog, to: root.appendingPathComponent("Contents.json"))

struct MacIcon {
    var size: Int
    var scale: Int
    var filename: String
}

let icons: [MacIcon] = [
    .init(size: 16, scale: 1, filename: "icon-16.png"),
    .init(size: 16, scale: 2, filename: "icon-16@2x.png"),
    .init(size: 32, scale: 1, filename: "icon-32.png"),
    .init(size: 32, scale: 2, filename: "icon-32@2x.png"),
    .init(size: 128, scale: 1, filename: "icon-128.png"),
    .init(size: 128, scale: 2, filename: "icon-128@2x.png"),
    .init(size: 256, scale: 1, filename: "icon-256.png"),
    .init(size: 256, scale: 2, filename: "icon-256@2x.png"),
    .init(size: 512, scale: 1, filename: "icon-512.png"),
    .init(size: 512, scale: 2, filename: "icon-512@2x.png"),
]

for icon in icons {
    let pixels = icon.size * icon.scale
    let rep = makeRep(pixels: pixels, opaque: true) { canvas in
        drawMark(canvas: canvas, color: ink, filled: false, compact: pixels <= 32)
    }
    try writePNG(rep, to: iconDir.appendingPathComponent(icon.filename))
}

var iconEntries: [[String: Any]] = icons.map { icon in
    [
        "filename": icon.filename,
        "idiom": "mac",
        "scale": "\(icon.scale)x",
        "size": "\(icon.size)x\(icon.size)"
    ]
}
try writeJSON(
    ["images": iconEntries, "info": ["author": "xcode", "version": 1]],
    to: iconDir.appendingPathComponent("Contents.json")
)

func writeMenuSet(dir: URL, filled: Bool) throws {
    let names = ["LeashMenu.png", "LeashMenu@2x.png", "LeashMenu@3x.png"]
    let scales = [1, 2, 3]
    let point = 18
    for (name, scale) in zip(names, scales) {
        let pixels = point * scale
        let rep = makeRep(pixels: pixels, opaque: false) { canvas in
            drawMark(canvas: canvas, color: .black, filled: filled)
        }
        try writePNG(rep, to: dir.appendingPathComponent(name))
    }
    let images: [[String: Any]] = zip(names, scales).map { name, scale in
        [
            "filename": name,
            "idiom": "universal",
            "scale": "\(scale)x"
        ]
    }
    try writeJSON(
        [
            "images": images,
            "info": ["author": "xcode", "version": 1],
            "properties": [
                "template-rendering-intent": "template",
                "preserves-vector-representation": false
            ]
        ],
        to: dir.appendingPathComponent("Contents.json")
    )
}

try writeMenuSet(dir: outlineDir, filled: false)
try writeMenuSet(dir: filledDir, filled: true)

let shots = URL(fileURLWithPath: FileManager.default.currentDirectoryPath).appendingPathComponent("docs/shots")
try FileManager.default.createDirectory(at: shots, withIntermediateDirectories: true)
try writePNG(
    makeRep(pixels: 256, opaque: true) { canvas in
        drawMark(canvas: canvas, color: ink, filled: false, compact: false)
    },
    to: shots.appendingPathComponent("icon.png")
)

print("wrote \(root.path)")
