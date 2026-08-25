// Command server exposes the QR generator as an HTTP API.
//
//	POST /api/qr  multipart/form-data: value (required), logo (file, optional)
//	              + optional fields: size, format, style, fg, bg, ec,
//	              logo_scale, margin, validate, download
//	GET  /api/qr  same options as query params (no logo upload)
//	GET  /        demo page
//	GET  /healthz liveness probe
package main

import (
	"log"
	"net/http"
	"os"

	"qr-go1/server"
)

func main() {
	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("qr-go1 listening on %s", addr)
	if err := http.ListenAndServe(addr, server.New()); err != nil {
		log.Fatal(err)
	}
}
