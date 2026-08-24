package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shrl.io/io-shrl/internal/redisutil"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	redisAddr := envOr("SHRL_REDIS_ADDR", "localhost:6379")
	rdb := redisutil.Connect(context.Background(), redisAddr)
	defer rdb.Close()

	log.Println("worker started (stub): analytics processing is not yet implemented; " +
		"the 'visits' stream is accumulating for a later slice")
	// Idle until signalled; a later analytics slice replaces this.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
