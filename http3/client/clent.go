package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"log"
)

func main() {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	ctx := context.Background()
	session, err := quic.DialAddr(ctx, "localhost:4242", tlsConfig, nil)
	if err != nil {
		log.Fatal(err)
	}

	stream, err := session.OpenStreamSync(ctx)
	if err != nil {
		log.Fatal(err)
	}

	message := "Hello from client"
	fmt.Printf("Client sending: %s\n", message)
	_, err = stream.Write([]byte(message))
	if err != nil {
		log.Fatal(err)
	}
	stream.Close()
	response, err := io.ReadAll(stream)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Client received: %s\n", string(response))
}
