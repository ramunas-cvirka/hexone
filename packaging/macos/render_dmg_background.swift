import AppKit
import Foundation

func makeParagraph(_ text: String, font: NSFont, color: NSColor, alignment: NSTextAlignment = .center) -> NSAttributedString {
    let style = NSMutableParagraphStyle()
    style.alignment = alignment
    style.lineBreakMode = .byTruncatingTail
    return NSAttributedString(string: text, attributes: [
        .font: font,
        .foregroundColor: color,
        .paragraphStyle: style,
    ])
}

guard CommandLine.arguments.count >= 3 else {
    fputs("usage: render_dmg_background.swift <output.png> <app-name>\n", stderr)
    exit(2)
}

let outputPath = CommandLine.arguments[1]
let appName = CommandLine.arguments.dropFirst(2).joined(separator: " ").trimmingCharacters(in: .whitespacesAndNewlines)
let title = appName.isEmpty ? "Hexone" : appName
let subtitle = "Drag into Applications to install \(title)"

let size = NSSize(width: 640, height: 420)
let image = NSImage(size: size)
image.lockFocus()

let gradient = NSGradient(colors: [
    NSColor(calibratedRed: 0.97, green: 0.98, blue: 1.00, alpha: 1),
    NSColor(calibratedRed: 0.89, green: 0.92, blue: 0.97, alpha: 1),
])!
gradient.draw(in: NSRect(origin: .zero, size: size), angle: -90)

let topGlow = NSBezierPath(ovalIn: NSRect(x: -80, y: 270, width: 800, height: 210))
NSColor(calibratedRed: 0.72, green: 0.82, blue: 0.98, alpha: 0.22).setFill()
topGlow.fill()

let leftWell = NSBezierPath(roundedRect: NSRect(x: 76, y: 92, width: 188, height: 174), xRadius: 42, yRadius: 42)
NSColor(calibratedWhite: 1.0, alpha: 0.58).setFill()
leftWell.fill()

let rightWell = NSBezierPath(roundedRect: NSRect(x: 376, y: 92, width: 188, height: 174), xRadius: 42, yRadius: 42)
NSColor(calibratedWhite: 1.0, alpha: 0.58).setFill()
rightWell.fill()

let leftWellBorder = NSBezierPath(roundedRect: NSRect(x: 76.5, y: 92.5, width: 187, height: 173), xRadius: 41.5, yRadius: 41.5)
leftWellBorder.lineWidth = 1
NSColor(calibratedRed: 0.63, green: 0.70, blue: 0.82, alpha: 0.22).setStroke()
leftWellBorder.stroke()

let rightWellBorder = NSBezierPath(roundedRect: NSRect(x: 376.5, y: 92.5, width: 187, height: 173), xRadius: 41.5, yRadius: 41.5)
rightWellBorder.lineWidth = 1
NSColor(calibratedRed: 0.63, green: 0.70, blue: 0.82, alpha: 0.22).setStroke()
rightWellBorder.stroke()

let separator = NSBezierPath()
separator.move(to: NSPoint(x: 56, y: 302))
separator.line(to: NSPoint(x: size.width - 56, y: 302))
separator.lineWidth = 1
NSColor(calibratedRed: 0.36, green: 0.45, blue: 0.60, alpha: 0.16).setStroke()
separator.stroke()

let titleFont = NSFont.systemFont(ofSize: 38, weight: .medium)
let footerFont = NSFont.systemFont(ofSize: 14, weight: .regular)
let arrowFont = NSFont.systemFont(ofSize: 72, weight: .ultraLight)

let titleAttr = makeParagraph(title, font: titleFont, color: NSColor(calibratedRed: 0.10, green: 0.14, blue: 0.22, alpha: 0.94))
titleAttr.draw(in: NSRect(x: 60, y: 328, width: size.width - 120, height: 48))

let arrowAttr = makeParagraph("→", font: arrowFont, color: NSColor(calibratedRed: 0.31, green: 0.40, blue: 0.56, alpha: 0.64))
arrowAttr.draw(in: NSRect(x: 270, y: 126, width: 100, height: 84))

let footerAttr = makeParagraph(subtitle, font: footerFont, color: NSColor(calibratedRed: 0.20, green: 0.25, blue: 0.34, alpha: 0.74))
footerAttr.draw(in: NSRect(x: 74, y: 34, width: size.width - 148, height: 22))

image.unlockFocus()

guard
    let tiff = image.tiffRepresentation,
    let rep = NSBitmapImageRep(data: tiff),
    let png = rep.representation(using: .png, properties: [:])
else {
    fputs("failed to render dmg background image\n", stderr)
    exit(1)
}

do {
    try png.write(to: URL(fileURLWithPath: outputPath))
} catch {
    fputs("failed to write \(outputPath): \(error)\n", stderr)
    exit(1)
}
