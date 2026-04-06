import SwiftUI

/// Airlock brand icon: space airlock hatch front view, black background with white glow.
struct AirlockIconView: View {
    var size: CGFloat = 128

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
            // Glow
            context.stroke(outerCircle, with: .color(w.opacity(0.04)),
                           style: StrokeStyle(lineWidth: drawSize * 0.08))
            context.stroke(outerCircle, with: .color(w.opacity(0.08)),
                           style: StrokeStyle(lineWidth: drawSize * 0.045))
            // Core
            context.stroke(outerCircle, with: .color(w.opacity(0.85)),
                           style: StrokeStyle(lineWidth: drawSize * 0.025))

            // === RIM TABS / BRACKETS — 12 protruding tabs ===
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

            // === 4 RADIAL SPOKES (X pattern, rotated ~40deg) ===
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
                // Glow
                context.stroke(spoke, with: .color(w.opacity(0.06)),
                               style: StrokeStyle(lineWidth: spokeWidth * 2.5, lineCap: .butt))
                // Core
                context.stroke(spoke, with: .color(w.opacity(0.7)),
                               style: StrokeStyle(lineWidth: spokeWidth, lineCap: .butt))
            }

            // === LATCH CLAMPS at spoke ends ===
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
            // Glow
            context.stroke(hubCircle, with: .color(w.opacity(0.06)),
                           style: StrokeStyle(lineWidth: drawSize * 0.05))
            context.stroke(hubCircle, with: .color(w.opacity(0.10)),
                           style: StrokeStyle(lineWidth: drawSize * 0.03))
            // Core
            context.stroke(hubCircle, with: .color(w.opacity(0.80)),
                           style: StrokeStyle(lineWidth: drawSize * 0.016))

            // Hub center dot
            let dotR = drawSize * 0.02
            let dot = Path(ellipseIn: CGRect(
                x: center.x - dotR, y: center.y - dotR,
                width: dotR * 2, height: dotR * 2
            ))
            context.fill(dot, with: .color(w.opacity(0.6)))

            // === HANDLE (small bar, left of center, ~170 deg) ===
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

            // === CONTROL PANEL (small rect, right of center, ~350 deg) ===
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

    /// Build a rotated rectangle path. The tangent axis (lx) is perpendicular to
    /// the radial direction, the radial axis (ly) points outward along the angle.
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

// MARK: - NSImage generation for dock icon

#if canImport(AppKit)
import AppKit

extension AirlockIconView {
    @MainActor
    static func makeNSImage(size: CGFloat = 512) -> NSImage {
        let renderer = ImageRenderer(content: AirlockIconView(size: size))
        renderer.scale = 2.0
        guard let cgImage = renderer.cgImage else {
            return NSImage(systemSymbolName: "lock.shield", accessibilityDescription: "Airlock")
                ?? NSImage(size: NSSize(width: size, height: size))
        }
        return NSImage(cgImage: cgImage, size: NSSize(width: size, height: size))
    }
}
#endif
