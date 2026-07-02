package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sonar-remote/internal/mdnsx"
	"sonar-remote/internal/sonar"
)

//go:embed all:web/dist
var embeddedWeb embed.FS

func main() {
	port := flag.Int("port", 80, "HTTP port to listen on (80 = bare http://<host>.local)")
	host := flag.String("host", "sonar", "mDNS hostname (published as <host>.local)")
	noMDNS := flag.Bool("no-mdns", false, "disable mDNS advertising")
	flag.Parse()

	// Windows' MIME registry is unreliable; pin the types we serve.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
	mime.AddExtensionType(".svg", "image/svg+xml")
	mime.AddExtensionType(".js", "text/javascript")
	mime.AddExtensionType(".css", "text/css")

	client := sonar.NewClient()
	client.EnsureDiscovered()

	mux := http.NewServeMux()
	mux.Handle("/api/", client.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if b := client.Base(); b != nil {
			fmt.Fprintf(w, `{"status":"ok","sonar":%q}`, b.String())
			return
		}
		http.Error(w, `{"status":"discovering"}`, http.StatusServiceUnavailable)
	})
	mux.Handle("/", spaHandler())

	if !*noMDNS {
		if stop, method, err := mdnsx.Advertise(*host, *port); err != nil {
			log.Printf("mDNS disabled: %v", err)
		} else {
			defer stop()
			log.Printf("mDNS: advertising %s.local via %s", *host, method)
		}
	}

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(*port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		printBanner(*host, *port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Println("shutting down…")
}

// spaHandler serves the embedded Svelte build, falling back to index.html.
func spaHandler() http.Handler {
	sub, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		log.Fatalf("embed: %v", err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func printBanner(host string, port int) {
	suffix := ""
	if port != 80 {
		suffix = ":" + strconv.Itoa(port)
	}
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║           Sonar Remote — en ligne          ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Printf("   • Local     : http://localhost%s\n", suffix)
	fmt.Printf("   • mDNS      : http://%s.local%s\n", host, suffix)
	for _, ip := range mdnsx.LocalIPv4() {
		fmt.Printf("   • Réseau    : http://%s%s\n", ip, suffix)
	}
	fmt.Println()
	log.Println("ready")
}
