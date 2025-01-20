package server

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type ServerConfiguration struct {
	UseTls   bool
	CertFile string
	KeyFile  string
}

// Starts serving requests on a specified port with graceful shutdown support
// Blocks the calling thread
// port is a string in Gin format, e.g. ":8600"
//
//	when port is an empty string, serves on default HTTP port
//
// config allows to configure https
// callback is called after the server has been setup to serve
//
//	callback is passed the actual port the server is listening on
func Serve(router *gin.Engine, port string, config *ServerConfiguration, callback func()) {
	// based on example from https://github.com/gin-gonic/examples
	ctx, restoreInterrupt := getNotifyContextForInterruptSignals()
	defer restoreInterrupt()

	httpServer := startServingAsync(router, port, config)
	if callback != nil {
		callback()
	}

	waitForInterruptSignal(ctx)
	restoreInterrupt()
	shutDownWithTimeout(httpServer, 5*time.Second)
}

func getNotifyContextForInterruptSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

func waitForInterruptSignal(ctx context.Context) {
	<-ctx.Done()
}

func startServingAsync(router *gin.Engine, port string, config *ServerConfiguration) *http.Server {
	log.Printf("Starting server on port %s (TLS: %v)", port, config.UseTls)

	httpServer := &http.Server{
		Addr:    port,
		Handler: router,
	}

	if config.UseTls {
		go listenAndServeTLS(httpServer, config.CertFile, config.KeyFile)
	} else {
		go listenAndServe(httpServer)
	}

	return httpServer
}

func listenAndServe(httpServer *http.Server) {
	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error serving: %s\n", err)
	}
}

func listenAndServeTLS(httpServer *http.Server, certFile string, keyFile string) {
	err := httpServer.ListenAndServeTLS(certFile, keyFile)
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error serving with TLS: %s\n", err)
	}
}

func shutDownWithTimeout(httpServer *http.Server, timeout time.Duration) {
	log.Println("Shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
