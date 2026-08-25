import Vision
import AppKit

let url = URL(fileURLWithPath: CommandLine.arguments[1])
guard let img = NSImage(contentsOf: url),
      let cg = img.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
    print("ERROR: cannot load image"); exit(1)
}
let req = VNDetectBarcodesRequest()
req.symbologies = [.qr]
let handler = VNImageRequestHandler(cgImage: cg)
do {
    try handler.perform([req])
} catch {
    print("ERROR: \(error)"); exit(1)
}
let results = req.results ?? []
if results.isEmpty { print("NO_QR_FOUND"); exit(2) }
for r in results { print("DECODED: \(r.payloadStringValue ?? "<binary>")") }
