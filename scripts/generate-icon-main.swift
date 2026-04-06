// generate-icon-main.swift
//
// Renders the Airlock dock icon to PNG files at all sizes required for a
// macOS .icns file. Run via scripts/generate-icon.sh, which then calls
// iconutil -c icns to assemble the final AppIcon.icns.
//
// NOTE: The Canvas drawing logic here is duplicated from
// AirlockApp/Sources/AirlockApp/Views/AppIconView.swift because that file
// lives in an @main SPM executable target and cannot be imported as a
// library. Any visual changes to the icon must be made in BOTH files.

import AppKit
import SwiftUI

struct AirlockIcon: View {
    var size: CGFloat

    var body: some View {
        Canvas { context, canvasSize in
            let center = CGPoint(x: canvasSize.width / 2, y: canvasSize.height / 2)
            let inset = canvasSize.width * 0.08
            let drawSize = canvasSize.width - inset * 2
            let w = Color.white

            // --- Black rounded background ---
            let bgRect = CGRect(x: inset, y: inset, width: drawSize, height: drawSize)
            let bgPath = Path(roundedRect: bgRect, cornerRadius: drawSize * 0.22)
            context.fill(bgPath, with: .color(.black))

            let outerR = drawSize * 0.42
            let innerR = drawSize * 0.34
            let hubR = drawSize * 0.10

            // === OUTER RIM ===
            let outerCircle = Path(ellipseIn: CGRect(
                x: center.x - outerR, y: center.y - outerR,
                width: outerR * 2, height: outerR * 2
            ))
            context.stroke(outerCircle, with: .color(w.opacity(0.04)),
                           style: StrokeStyle(lineWidth: drawSize * 0.08))
            context.stroke(outerCircle, with: .color(w.opacity(0.08)),
                           style: StrokeStyle(lineWidth: drawSize * 0.045))
            context.stroke(outerCircle, with: .color(w.opacity(0.85)),
                           style: StrokeStyle(lineWidth: drawSize * 0.025))

            // === RIM TABS — 12 protruding tabs ===
            let tabCount = 12
            let tabInnerR = outerR + drawSize * 0.008
            let tabOuterR = outerR + drawSize * 0.04
            let tabWidth = drawSize * 0.025
            for i in 0..<tabCount {
                let angle = Double(i) * (360.0 / Double(tabCount)) + 15.0
                let rad = angle * .pi / 180.0
                let tabRadius: CGFloat = (tabInnerR + tabOuterR) / 2
                let tabCenter = CGPoint(
                    x: center.x + tabRadius * cos(rad),
                    y: center.y + tabRadius * sin(rad)
                )
                let tabH = tabOuterR - tabInnerR
                let tab = Self.rotatedRect(center: tabCenter, halfWidth: tabWidth / 2, halfHeight: tabH / 2, angle: rad)
                context.fill(tab, with: .color(w.opacity(0.6)))
            }

            // === INNER CHANNEL RING ===
            let innerCircle = Path(ellipseIn: CGRect(
                x: center.x - innerR, y: center.y - innerR,
                width: innerR * 2, height: innerR * 2
            ))
            context.stroke(innerCircle, with: .color(w.opacity(0.03)),
                           style: StrokeStyle(lineWidth: drawSize * 0.03))
            context.stroke(innerCircle, with: .color(w.opacity(0.25)),
                           style: StrokeStyle(lineWidth: drawSize * 0.008))

            // === 4 RADIAL SPOKES ===
            let spokeAngles: [Double] = [40, 130, 220, 310]
            let spokeInnerR = hubR + drawSize * 0.012
            let spokeOuterR = innerR - drawSize * 0.01
            let spokeWidth = drawSize * 0.028

            for angle in spokeAngles {
                let rad = angle * .pi / 180.0
                let from = CGPoint(
                    x: center.x + spokeInnerR * cos(rad),
                    y: center.y + spokeInnerR * sin(rad)
                )
                let to = CGPoint(
                    x: center.x + spokeOuterR * cos(rad),
                    y: center.y + spokeOuterR * sin(rad)
                )
                var spoke = Path()
                spoke.move(to: from)
                spoke.addLine(to: to)
                context.stroke(spoke, with: .color(w.opacity(0.06)),
                               style: StrokeStyle(lineWidth: spokeWidth * 2.5, lineCap: .butt))
                context.stroke(spoke, with: .color(w.opacity(0.7)),
                               style: StrokeStyle(lineWidth: spokeWidth, lineCap: .butt))
            }

            // === LATCH CLAMPS ===
            let clampR = innerR + drawSize * 0.005
            let clampW = drawSize * 0.042
            let clampH = drawSize * 0.022
            for angle in spokeAngles {
                let rad = angle * .pi / 180.0
                let cc = CGPoint(
                    x: center.x + clampR * cos(rad),
                    y: center.y + clampR * sin(rad)
                )
                let clamp = Self.rotatedRect(center: cc, halfWidth: clampW / 2, halfHeight: clampH / 2, angle: rad)
                context.fill(clamp, with: .color(w.opacity(0.75)))
            }

            // === CENTRAL HUB ===
            let hubCircle = Path(ellipseIn: CGRect(
                x: center.x - hubR, y: center.y - hubR,
                width: hubR * 2, height: hubR * 2
            ))
            context.stroke(hubCircle, with: .color(w.opacity(0.06)),
                           style: StrokeStyle(lineWidth: drawSize * 0.05))
            context.stroke(hubCircle, with: .color(w.opacity(0.10)),
                           style: StrokeStyle(lineWidth: drawSize * 0.03))
            context.stroke(hubCircle, with: .color(w.opacity(0.80)),
                           style: StrokeStyle(lineWidth: drawSize * 0.016))

            let dotR = drawSize * 0.02
            let dot = Path(ellipseIn: CGRect(
                x: center.x - dotR, y: center.y - dotR,
                width: dotR * 2, height: dotR * 2
            ))
            context.fill(dot, with: .color(w.opacity(0.6)))

            // === HANDLE ===
            let handleAngle = 175.0 * .pi / 180.0
            let handleDist = hubR * 1.6
            let handleCenter = CGPoint(
                x: center.x + handleDist * cos(handleAngle),
                y: center.y + handleDist * sin(handleAngle)
            )
            let handleLen = drawSize * 0.04
            var handle = Path()
            handle.move(to: CGPoint(x: handleCenter.x, y: handleCenter.y - handleLen / 2))
            handle.addLine(to: CGPoint(x: handleCenter.x, y: handleCenter.y + handleLen / 2))
            context.stroke(handle, with: .color(w.opacity(0.6)),
                           style: StrokeStyle(lineWidth: drawSize * 0.012, lineCap: .round))

            // === CONTROL PANEL ===
            let panelAngle = 355.0 * .pi / 180.0
            let panelDist = hubR * 1.7
            let panelCenter = CGPoint(
                x: center.x + panelDist * cos(panelAngle),
                y: center.y + panelDist * sin(panelAngle)
            )
            let panelW = drawSize * 0.035
            let panelH = drawSize * 0.028
            let panelRect = CGRect(
                x: panelCenter.x - panelW / 2, y: panelCenter.y - panelH / 2,
                width: panelW, height: panelH
            )
            let panelPath = Path(roundedRect: panelRect, cornerRadius: drawSize * 0.004)
            context.stroke(panelPath, with: .color(w.opacity(0.5)),
                           style: StrokeStyle(lineWidth: drawSize * 0.006))
        }
        .frame(width: size, height: size)
    }

    private static func rotatedRect(center: CGPoint, halfWidth hw: CGFloat, halfHeight hh: CGFloat, angle: Double) -> Path {
        let cosA = cos(angle)
        let sinA = sin(angle)
        let corners: [(CGFloat, CGFloat)] = [
            (-hw, -hh), (hw, -hh), (hw, hh), (-hw, hh),
        ]
        var path = Path()
        for (i, (lx, ly)) in corners.enumerated() {
            let pt = CGPoint(
                x: center.x + lx * (-sinA) + ly * cosA,
                y: center.y + lx * cosA + ly * sinA
            )
            if i == 0 { path.move(to: pt) } else { path.addLine(to: pt) }
        }
        path.closeSubpath()
        return path
    }
}

// MARK: - Rendering

@MainActor
func renderPNG(pixelSize: Int, to url: URL) throws {
    let pointSize = CGFloat(pixelSize)
    let renderer = ImageRenderer(content: AirlockIcon(size: pointSize))
    renderer.scale = 1.0

    guard let cgImage = renderer.cgImage else {
        throw NSError(domain: "generate-icon", code: 1,
                      userInfo: [NSLocalizedDescriptionKey: "Failed to render CGImage at size \(pixelSize)"])
    }

    let bitmapRep = NSBitmapImageRep(cgImage: cgImage)
    bitmapRep.size = NSSize(width: pointSize, height: pointSize)

    guard let pngData = bitmapRep.representation(using: .png, properties: [:]) else {
        throw NSError(domain: "generate-icon", code: 2,
                      userInfo: [NSLocalizedDescriptionKey: "Failed to encode PNG at size \(pixelSize)"])
    }

    try pngData.write(to: url)
}

// MARK: - Main

// Usage: generate-icon-main <output-iconset-dir>
guard CommandLine.arguments.count == 2 else {
    FileHandle.standardError.write("Usage: generate-icon-main <output-iconset-dir>\n".data(using: .utf8)!)
    exit(1)
}

let outputDir = URL(fileURLWithPath: CommandLine.arguments[1])

do {
    try FileManager.default.createDirectory(at: outputDir, withIntermediateDirectories: true)

    // Required .icns sizes: each entry is (filename, pixel size)
    let entries: [(String, Int)] = [
        ("icon_16x16.png", 16),
        ("icon_16x16@2x.png", 32),
        ("icon_32x32.png", 32),
        ("icon_32x32@2x.png", 64),
        ("icon_128x128.png", 128),
        ("icon_128x128@2x.png", 256),
        ("icon_256x256.png", 256),
        ("icon_256x256@2x.png", 512),
        ("icon_512x512.png", 512),
        ("icon_512x512@2x.png", 1024),
    ]

    try MainActor.assumeIsolated {
        for (name, pixels) in entries {
            let fileURL = outputDir.appendingPathComponent(name)
            try renderPNG(pixelSize: pixels, to: fileURL)
            print("wrote \(name) (\(pixels)px)")
        }
    }
} catch {
    FileHandle.standardError.write("Error: \(error.localizedDescription)\n".data(using: .utf8)!)
    exit(1)
}
