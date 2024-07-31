package main

import (
	"fmt"
	"github.com/txthinking/socks5"
	"math"
)

func main() {
	c, err := socks5.NewClient("localhost:1080", "rrr", "123123", 1, 1)
	if err != nil {
		panic(err)
	}

	defer c.Close()
	fmt.Println(math.MaxInt32)
}
