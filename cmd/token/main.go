package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/auth"
)

func main() {
	sub := flag.String("sub", "", "principal UUID (default: random)")
	ttl := flag.Duration("ttl", 1*time.Hour, "token lifetime")
	flag.Parse()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	priv, err := auth.LoadPrivateKey(dir)
	if err != nil {
		log.Fatal("private key error:", err)
	}

	id := uuid.New()
	if *sub != "" {
		id, err = uuid.Parse(*sub)
		if err != nil {
			log.Fatalf("sub error: %v", err)
		}
	}

	raw, err := auth.TokenTTL(priv, id, *ttl)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(raw)
}
