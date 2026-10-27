package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed assets/*
var assets embed.FS

const version = "1.0.15"

func main() {
	dir := flag.String("data-dir", "/opt/etc/optiray-go", "settings directory")
	addr := flag.String("listen", ":3883", "HTTP address")
	show := flag.Bool("version", false, "print version")
	enableCore := flag.Bool("enable-core", false, "allow this test instance to launch its own Xray")
	xrayBin := flag.String("xray-bin", "", "Xray executable; default data-dir/bin/xray")
	hyBin := flag.String("hysteria-bin", "", "Hysteria2 executable; default data-dir/bin/hysteria")
	hy1Bin := flag.String("hysteria1-bin", "", "Hysteria1 executable; default data-dir/bin/hysteria1")
	uiDir := flag.String("ui-dir", "", "external HTML/CSS/JS directory; empty uses embedded assets")
	enableKeenetic := flag.Bool("enable-keenetic", false, "allow proxy configuration changes on Keenetic")
	ndmcBin := flag.String("ndmc-bin", "/bin/ndmc", "Keenetic command executable")
	enableRouting := flag.Bool("enable-routing", false, "allow owned OptiRout process startup")
	routingBin := flag.String("routing-bin", "", "OptiRout executable; default data-dir/bin/optiroutd")
	hook := flag.Bool("netfilter-hook", false, "forward a Keenetic firewall rebuild to the owned routing socket")
	hookType := flag.String("hook-type", "", "firewall family from firmware hook")
	hookTable := flag.String("hook-table", "", "firewall table from firmware hook")
	removeProxy := flag.Bool("remove-managed-proxy", false, "restore only journaled Keenetic proxy interfaces (service must be stopped)")

	routerAddress := flag.Bool("router-address", false, "print detected LAN bridge IPv4")
	initialize := flag.Bool("initialize-install", false, "persist defaults with the detected LAN address on a fresh installation")
	checkService := flag.Bool("check-service", false, "check the owned installed HTTP service and routing/core readiness")
	checkInstallation := flag.Bool("check-installation", false, "read-only validation of saved configuration and installed cores")
	serviceAddress := flag.String("service-address", "127.0.0.1:3883", "local health check address")
	stopLegacy := flag.Bool("stop-legacy", false, "stop only verified legacy OptiRay processes")
	legacyStatus := flag.Bool("legacy-status", false, "check for verified legacy OptiRay processes")
	captureLegacyAuth := flag.Bool("capture-legacy-auth", false, "preserve active legacy environment authentication without printing credentials")
	startLegacy := flag.Bool("start-legacy", false, "restore the verified legacy installation after a failed migration")
	flag.Parse()
	if *show {
		fmt.Println(version)
		return
	}
	absoluteData, e := filepath.Abs(*dir)
	if e != nil {
		log.Fatal(e)
	}
	*dir = absoluteData
	if *checkInstallation {
		if e := checkInstallationFiles(context.Background(), *dir); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *hook {
		if e := netfilterHook(context.Background(), *dir, *hookType, *hookTable); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *removeProxy {
		k := &Keenetic{dir: filepath.Join(*dir, "install-state"), run: executableNDMC(*ndmcBin)}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if e := k.RemoveManaged(ctx); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *routerAddress || *initialize {
		address, e := routerLANAddress()
		if e != nil {
			log.Fatal(e)
		}
		if *routerAddress {
			fmt.Println(address)
			return
		}
		if e = initializeInstallation(*dir, address); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *checkService {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if e := waitInstalledService(ctx, *dir, *serviceAddress, time.Second); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *legacyStatus {
		if legacyRunning() {
			fmt.Println("running")
		} else {
			fmt.Println("stopped")
		}
		return
	}
	if *stopLegacy {
		if e := stopLegacyInstallation(); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *captureLegacyAuth {
		if e := captureLegacyAuthentication(*dir); e != nil {
			log.Fatal(e)
		}
		return
	}
	if *startLegacy {
		if e := startLegacyInstallation(*dir); e != nil {
			log.Fatal(e)
		}
		return
	}
	configureEntwareCertificates("/opt/etc/ssl/certs/ca-certificates.crt")
	store, e := newStore(*dir)
	if e != nil {
		log.Fatal(e)
	}
	log.SetOutput(newDiagnosticWriter(*dir, "log/manager.log"))
	listener, e := net.Listen("tcp", *addr)
	if e != nil {
		log.Fatalf("HTTP listen failed: %v", e)
	}
	defer listener.Close()
	app := &App{store: store, uiDir: *uiDir}
	if *enableRouting {
		absoluteDir, e := filepath.Abs(*dir)
		if e != nil {
			log.Fatal(e)
		}
		binary := *routingBin
		if binary == "" {
			binary = filepath.Join(absoluteDir, "bin", "optiroutd")
		}
		binary, e = filepath.Abs(binary)
		if e != nil {
			log.Fatal(e)
		}
		app.routing = &RoutingProcess{bin: binary, dir: absoluteDir, productionPID: "/opt/var/run/optirout.pid"}
		defer func() {
			if e := app.routing.Stop(); e != nil {
				log.Printf("Routing shutdown failed: %v", e)
				_ = atomicBytes(filepath.Join(*dir, "shutdown-error"), []byte(e.Error()))
			}
		}()
	}
	if *enableKeenetic {
		app.keenetic = &Keenetic{dir: filepath.Join(*dir, "install-state"), run: executableNDMC(*ndmcBin)}
	}
	if *enableCore {
		if *xrayBin == "" {
			*xrayBin = filepath.Join(*dir, "bin", "xray")
		}
		if *hyBin == "" {
			*hyBin = filepath.Join(*dir, "bin", "hysteria")
		}
		if *hy1Bin == "" {
			*hy1Bin = filepath.Join(*dir, "bin", "hysteria1")
		}
		app.core = &CoreManager{bin: *xrayBin, dir: *dir, hysteriaBin: *hyBin, hysteria1Bin: *hy1Bin}
		defer app.core.Stop()
	}
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	defer runtimeCancel()
	go app.supervise(runtimeCtx)
	go store.superviseCoreReleaseSchedule(runtimeCtx)
	srv := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		runtimeCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	log.Printf("OptiRay %s listening on %s; data: %s", version, *addr, *dir)
	if e = srv.Serve(listener); e != nil && e != http.ErrServerClosed {
		log.Printf("HTTP server stopped: %v", e)
	}
}
