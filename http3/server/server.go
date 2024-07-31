package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"net"
)

func generateTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("./cert/cert.pem", "./cert/key.pem")
	if err != nil {
		log.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func main() {
	// 创建一个简单的HTTP处理程序
	testQuic()
}

func testQuic() {
	tlsConfig := generateTLSConfig()

	listener, err := quic.ListenAddr("localhost:8888", tlsConfig, &quic.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Server listening on localhost:8888")

	for {
		ctx := context.Background()
		sess, err := listener.Accept(ctx)
		if err != nil {
			log.Fatal(err)
		}

		go func(sess quic.Connection) {
			fmt.Println("new session")
			stream, err := sess.AcceptStream(ctx)
			if err != nil {
				log.Fatal(err)
			}
			addr := make([]byte, 64)
			_, err = stream.Read(addr)
			if err != nil {
				log.Println(err)
				return

			}

			for i, b := range addr {
				if rune(b) == rune('$') {
					addr = addr[:i]
				}
			}

			conn, err := net.Dial("tcp", string(addr))
			if err != nil {
				stream.Write([]byte{0x2})
				return
			}
			_, err = stream.Write([]byte{0x1})
			if err != nil {
				log.Println(err)
			}

			go func() {
				//defer conn.Close()

				buf := make([]byte, 4*1024)

				for {
					n, err1 := stream.Read(buf)

					_, err = conn.Write(buf[:n])
					if err != nil {
						return
					}
					if err1 != nil {
						return
					}
				}

			}()

			go func() {

				//defer conn.Close()
				buf := make([]byte, 4*1024)

				for {
					n, err1 := conn.Read(buf)

					_, err = stream.Write(buf[:n])
					if err != nil {
						return
					}
					if err1 != nil {
						return
					}
				}
			}()

		}(sess)
	}
}
