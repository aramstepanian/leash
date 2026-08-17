#!/usr/bin/env swift
import AppKit
import Foundation

// Clip-and-strap mark → App Icon + menu bar template images.
// Run from repo root: swift macos/render-assets.swift

let paper = NSColor(srgbRed: 0.957, green: 0.945, blue: 0.925, alpha: 1)
let ink = NSColor(srgbRed: 0.102, green: 0.098, blue: 0.086, alpha: 1)

struct Mark {
    var circle: CGRect
    var strap: CGRect
    var line: CGFloat
}

func layout(canvas: CGSize, padding: CGFloat) -> Mark {
    let inset = min(canvas.width, canvas.height) * padding
    let inner = CGRect(x: inset, y: inset, width: canvas.width - inset * 2, height: canvas.height - inset * 2)
    let line = max(canvas.height * 0.10, inner.height * 0.12)
    let gap = line * 0.50
    let strapW = max(line * 2.2, inner.width * 0.28)
    var diameter = min(inner.height, inner.width - gap - strapW)
    if diameter < inner.height * 0.55 {
        diameter = min(inner.height, inner.width * 0.62)
    }
    let fittedStrap = max(line * 2.0, inner.width - diameter - gap)
    let groupW = diameter + gap + fittedStrap
    let originX = inner.minX + max(0, (inner.width - groupW) / 2)
    let originY = inner.minY + (inner.height - diameter) / 2
    return Mark(
        circle: CGRect(x: originX, y: originY, width: diameter, height: diameter),
        strap: CGRect(
            x: originX + diameter + gap,
            y: originY + (diameter - line) / 2,
            width: fittedStrap,
            height: line
        ),
        line: line
    )
}

func strokeCircle(rect: CGRect, line: CGFloat, color: NSColor) {
    let path = NSBezierPath(ovalIn: rect.insetBy(dx: line / 2, dy: line / 2))
    color.setStroke()
    path.lineWidth = line
    path.lineCapStyle = .round
    path.stroke()
}

func fillCircle(in rect: CGRect, inset: CGFloat, color: NSColor) {
    let path = NSBezierPath(ovalIn: rect.insetBy(dx: inset, dy: inset))
    color.setFill()
    path.fill()
}

func fillStrap(_ rect: CGRect, color: NSColor) {
    let path = NSBezierPath(roundedRect: rect, xRadius: rect.height / 2, yRadius: rect.height / 2)
    color.setFill()
    path.fill()
}

func drawMark(canvas: CGSize, color: NSColor, filled: Bool, padding: CGFloat, compact: Bool = false) {
    let mark = layout(canvas: canvas, padding: padding)
    if compact {
        fillCircle(in: mark.circle, inset: 0, color: color)
        fillStrap(mark.strap, color: color)
        return
    }
    strokeCircle(rect: mark.circle, line: mark.line, color: color)
    if filled {
        fillCircle(in: mark.circle, inset: mark.line * 1.65, color: color)
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
        drawMark(canvas: canvas, color: ink, filled: true, padding: 0.22, compact: pixels <= 32)
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
            drawMark(canvas: canvas, color: .black, filled: filled, padding: 0.18)
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
        drawMark(canvas: canvas, color: ink, filled: true, padding: 0.22)
    },
    to: shots.appendingPathComponent("icon.png")
)

print("wrote \(root.path)")
