package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"log"
	"net"
	"runtime"
	"time"
)

func generateTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("./cert/cert.pem", "./cert/key.pem")
	if err != nil {
		log.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func main() {

	go monitorGoroutineNums()

	// 创建一个简单的HTTP处理程序
	runServer()
}

func monitorGoroutineNums() {
	for {
		time.Sleep(5 * time.Second)
		log.Printf("goroutine num:%d", runtime.NumGoroutine())
	}
}

func runServer() {
	tlsConfig := generateTLSConfig()

	listener, err := quic.ListenAddr("0.0.0.0:8888", tlsConfig, &quic.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Server listening on 0.0.0.0:8888")

	for {
		ctx := context.Background()
		sess, err := listener.Accept(ctx)
		if err != nil {
			log.Fatal(err)
		}

		go func(sess quic.Connection) {
			fmt.Println("new session")

			for {

				stream, err := sess.AcceptStream(ctx)
				if err != nil {
					log.Fatal(err)
				}
				go func() {

					addr := make([]byte, 64)
					_, err = stream.Read(addr)
					if err != nil {
						log.Println(err)
						return

					}

					defer stream.Close()

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

					defer conn.Close()

					_, err = stream.Write([]byte{0x1})
					if err != nil {
						log.Println(err)
						return
					}

					log.Printf("server: %s-->%s", conn.LocalAddr().String(), conn.RemoteAddr().String())

					streamChan := make(chan interface{}, 2)
					go func() {

						buf := make([]byte, 4*1024)

						for {
							n, err1 := stream.Read(buf)

							_, err = conn.Write(buf[:n])
							if err != nil {
								streamChan <- err
								return
							}
							if err1 != nil {
								streamChan <- err1
								return
							}
						}

					}()

					go func() {

						buf := make([]byte, 4*1024)

						for {
							n, err1 := conn.Read(buf)

							_, err = stream.Write(buf[:n])
							if err != nil {
								streamChan <- err
								return
							}
							if err1 != nil {
								streamChan <- err1
								return
							}
						}
					}()

					<-streamChan
					<-streamChan

				}()
			}
		}(sess)
	}
}
