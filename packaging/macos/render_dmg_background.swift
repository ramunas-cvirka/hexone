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
    NSColor(calibratedRed: 0.08, green: 0.09, blue: 0.12, alpha: 1),
    NSColor(calibratedRed: 0.05, green: 0.06, blue: 0.08, alpha: 1),
])!
gradient.draw(in: NSRect(origin: .zero, size: size), angle: -90)

let topGlow = NSBezierPath(ovalIn: NSRect(x: -40, y: 250, width: 720, height: 230))
NSColor(calibratedRed: 0.16, green: 0.19, blue: 0.28, alpha: 0.35).setFill()
topGlow.fill()

let iconGlowLeft = NSBezierPath(ovalIn: NSRect(x: 70, y: 105, width: 190, height: 150))
NSColor(calibratedRed: 0.18, green: 0.22, blue: 0.35, alpha: 0.22).setFill()
iconGlowLeft.fill()

let iconGlowRight = NSBezierPath(ovalIn: NSRect(x: 380, y: 105, width: 190, height: 150))
NSColor(calibratedRed: 0.12, green: 0.18, blue: 0.28, alpha: 0.18).setFill()
iconGlowRight.fill()

let leftLabelPlate = NSBezierPath(roundedRect: NSRect(x: 106, y: 104, width: 136, height: 38), xRadius: 19, yRadius: 19)
NSColor(calibratedRed: 0.90, green: 0.94, blue: 1.00, alpha: 0.26).setFill()
leftLabelPlate.fill()

let rightLabelPlate = NSBezierPath(roundedRect: NSRect(x: 392, y: 104, width: 176, height: 38), xRadius: 19, yRadius: 19)
NSColor(calibratedRed: 0.90, green: 0.94, blue: 1.00, alpha: 0.26).setFill()
rightLabelPlate.fill()

let leftLabelGlow = NSBezierPath(ovalIn: NSRect(x: 84, y: 90, width: 182, height: 66))
NSColor(calibratedRed: 0.74, green: 0.82, blue: 0.96, alpha: 0.14).setFill()
leftLabelGlow.fill()

let rightLabelGlow = NSBezierPath(ovalIn: NSRect(x: 374, y: 90, width: 212, height: 66))
NSColor(calibratedRed: 0.74, green: 0.82, blue: 0.96, alpha: 0.14).setFill()
rightLabelGlow.fill()

let separator = NSBezierPath()
separator.move(to: NSPoint(x: 56, y: 302))
separator.line(to: NSPoint(x: size.width - 56, y: 302))
separator.lineWidth = 1
NSColor(calibratedRed: 0.62, green: 0.69, blue: 0.82, alpha: 0.22).setStroke()
separator.stroke()

let titleFont = NSFont.systemFont(ofSize: 38, weight: .medium)
let footerFont = NSFont.systemFont(ofSize: 14, weight: .regular)
let arrowFont = NSFont.systemFont(ofSize: 72, weight: .ultraLight)

let titleAttr = makeParagraph(title, font: titleFont, color: NSColor(calibratedRed: 0.95, green: 0.96, blue: 0.99, alpha: 0.96))
titleAttr.draw(in: NSRect(x: 60, y: 328, width: size.width - 120, height: 48))

let arrowAttr = makeParagraph("→", font: arrowFont, color: NSColor(calibratedRed: 0.72, green: 0.76, blue: 0.84, alpha: 0.68))
arrowAttr.draw(in: NSRect(x: 270, y: 126, width: 100, height: 84))

let footerAttr = makeParagraph(subtitle, font: footerFont, color: NSColor(calibratedRed: 0.78, green: 0.81, blue: 0.87, alpha: 0.76))
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
