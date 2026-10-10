// Sonar Remote controls SteelSeries Sonar from the Windows tray and, over the
// LAN, from any browser (http://sonar.local).
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"strconv"
	"time"

	"sonar-remote/internal/desktop"
	"sonar-remote/internal/mdnsx"
	"sonar-remote/internal/sonar"
)

//go:embed all:web/dist
var embeddedWeb embed.FS

func main() {
	port := flag.Int("port", 80, "HTTP port of the web remote (80 = bare http://<host>.local)")
	host := flag.String("host", "sonar", "mDNS hostname (published as <host>.local)")
	noMDNS := flag.Bool("no-mdns", false, "disable mDNS advertising")
	noTray := flag.Bool("no-tray", false, "serve the web remote only, without the tray mixer")
	logFile := flag.String("log", "", "write logs and crash traces to this file (the release build has no console)")
	flag.Parse()

	if *logFile != "" {
		if f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			log.SetOutput(f)
			debug.SetCrashOutput(f, debug.CrashOptions{})
		}
	}

	client := sonar.NewClient()

	web, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		log.Fatalf("embed: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", client.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if b := client.Base(); b != nil {
			fmt.Fprintf(w, `{"status":"ok","sonar":%q}`, b.String())
			return
		}
		http.Error(w, `{"status":"discovering"}`, http.StatusServiceUnavailable)
	})
	mux.Handle("/", static(web))

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(*port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		// The tray mixer talks to Sonar directly, so it keeps working even if
		// the port is taken and the web remote can't start.
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("web remote unavailable: %v", err)
		}
	}()
	if !*noMDNS {
		defer mdnsx.Advertise(*host, *port)()
	}

	if *noTray {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
	} else {
		name := *host
		if *noMDNS {
			name = "" // not published, so not worth showing
		}
		if err := desktop.Run(client, name, *port); err != nil {
			log.Fatal(err)
		}
		log.Print("tray closed, exiting")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// contentTypes of the embedded files, set by hand so the mime package never
// reads Windows' MIME registry: it is unreliable (.js is often text/plain), and
// with Go 1.27.0 that read crashed the process at the next GC when it was
// started from Explorer or at logon rather than from a terminal.
var contentTypes = map[string]string{
	".html":        "text/html; charset=utf-8",
	".js":          "text/javascript",
	".css":         "text/css",
	".svg":         "image/svg+xml",
	".png":         "image/png",
	".webmanifest": "application/manifest+json",
}

func static(files fs.FS) http.Handler {
	server := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		if ext == "" {
			ext = ".html" // "/" serves index.html
		}
		ctype, ok := contentTypes[ext]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		server.ServeHTTP(w, r)
	})
}
