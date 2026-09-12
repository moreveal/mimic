package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/moreveal/mimic/chrome"
	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/cdp"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	quickjsengine "github.com/moreveal/mimic/internal/engine/quickjs"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/state"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9222", "CDP HTTP/WebSocket listen address")
	milestone := flag.Int("chrome", 152, "installed Chrome compatibility milestone")
	navigationTimeout := flag.Duration("navigation-timeout", 0, "optional navigation execution cap; 0 keeps loading until completion or cancellation")
	engineName := flag.String("engine", "v8", "ECMAScript engine adapter: v8, quickjs, or goja")
	browserMode := flag.String("browser-mode", "headful", "selected environment profile: headful or headless")
	profilePath := flag.String("profile", "", "JSON environment profile for new contexts")
	flag.Parse()
	bundle, err := chrome.GetForMode(*milestone, state.BrowserMode(*browserMode))
	if err != nil {
		log.Fatal(err)
	}
	var factory engine.Factory
	switch *engineName {
	case "quickjs":
		factory = quickjsengine.Factory{}
	case "goja":
		factory = gojaengine.Factory{}
	case "v8":
		factory = v8engine.Factory{}
	default:
		log.Fatalf("unsupported JavaScript engine %q", *engineName)
	}
	var profileJSON []byte
	if *profilePath != "" {
		profileJSON, err = os.ReadFile(*profilePath)
		if err != nil {
			log.Fatal(err)
		}
	}
	b, err := browser.NewWithOptions(factory, bundle, browser.Options{ProfileJSON: profileJSON})
	if err != nil {
		log.Fatal(err)
	}
	s, err := cdp.New(b)
	if err != nil {
		log.Fatal(err)
	}
	s.SetNavigationTimeout(*navigationTimeout)
	l, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Mimic listening on http://%s\n", l.Addr())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Close(shutdown)
	}()
	if err := s.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
	stop()
	<-shutdownDone
}
