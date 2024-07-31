package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/quic-go/quic-go"
	"github.com/txthinking/socks5"
	"io"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

const (
	pa = "localhost:8888"
)

func main() {

	socks5.Debug = true
	s, err := socks5.NewClassicServer(":1080", "localhost", "", "", 5, 5)
	if err != nil {
		panic(err)
	}

	hdl := &iHdl{
		qc: newSession(pa),
	}

	go func() {
		t := time.NewTicker(5 * time.Second)
		for {
			<-t.C
			i, o := hdl.State()
			log.Printf("inbound:%d, outbound:%d", i, o)
		}

	}()

	err = s.ListenAndServe(hdl)
	if err != nil {
		panic(err)
	}

}

func newSession(addr string) quic.Connection {

	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
	}
	session, err := quic.DialAddr(context.Background(), addr, tlsCfg, nil)
	if err != nil {
		panic(err)
	}
	return session
}

type iHdl struct {
	inBound  atomic.Int64
	outBound atomic.Int64

	qc quic.Connection
}

func (i *iHdl) State() (int64, int64) {
	return i.inBound.Load(), i.outBound.Load()
}

type firstReq struct {
	typ        string
	remoteAddr string
}

func (i *iHdl) TCPHandle(s *socks5.Server, c *net.TCPConn, r *socks5.Request) error {
	log.Printf("new tcp reqest: %s --> %v", c.RemoteAddr(), r.Address())

	ctx := context.Background()

	if r.Cmd == socks5.CmdConnect {
		rc, err := r.Connect(c)
		if err != nil {
			return err
		}
		defer rc.Close()

		stream, err := i.qc.OpenStreamSync(ctx)
		if err != nil {
			log.Printf("qc.OpenStreamSync|err=%v", err)
			return err
		}

		address := formatAddress(r.Address())
		_, err = stream.Write(address)
		if err != nil {
			log.Printf("stream.Write(address)|err=%v", err)
			return err
		}

		rsp := make([]byte, 1)

		_, err = stream.Read(rsp)
		if err != nil {
			return err
		}
		if rsp[0] != 0x1 {
			return nil
		}

		defer stream.Close()

		go func() {

			var bf [1024 * 2]byte
			for {
				//if s.TCPTimeout != 0 {
				//	if err := rc.SetDeadline(time.Now().Add(time.Duration(s.TCPTimeout) * time.Second)); err != nil {
				//		return
				//	}
				//}
				//n, err := rc.Read(bf[:])
				//if err != nil {
				//	return
				//}

				n, err := stream.Read(bf[:])
				if err != nil {
					return
				}

				i.outBound.Add(int64(n))
				if _, err := c.Write(bf[0:n]); err != nil {
					return
				}
			}
		}()
		var bf [1024 * 2]byte
		for {
			//if s.TCPTimeout != 0 {
			//	if err := c.SetDeadline(time.Now().Add(time.Duration(s.TCPTimeout) * time.Second)); err != nil {
			//		return nil
			//	}
			//}
			n, err := c.Read(bf[:])
			if err != nil {
				return nil
			}
			i.inBound.Add(int64(n))

			//
			//if _, err := rc.Write(bf[0:n]); err != nil {
			//	return nil
			//}

			if _, err = stream.Write(bf[0:n]); err != nil {
				log.Printf("stream.Write|err=%v", err)
				return nil
			}

		}
	}
	if r.Cmd == socks5.CmdUDP {
		caddr, err := r.UDP(c, s.ServerAddr)
		if err != nil {
			return err
		}
		ch := make(chan byte)
		defer close(ch)
		s.AssociatedUDP.Set(caddr.String(), ch, -1)
		defer s.AssociatedUDP.Delete(caddr.String())
		io.Copy(io.Discard, c)
		if socks5.Debug {
			log.Printf("A tcp connection that udp %#v associated closed\n", caddr.String())
		}
		return nil
	}
	return socks5.ErrUnsupportCmd
}

// 64 个字节的地址信息
func formatAddress(addr string) []byte {
	addr += "$"
	addrByte := []byte(addr)
	maxLen := 64
	if len(addrByte) > maxLen {
		return nil
	}
	addrs := make([]byte, maxLen)
	copy(addrs[:len(addrByte)], addrByte)
	return addrs
}

func (i *iHdl) UDPHandle(s *socks5.Server, addr *net.UDPAddr, d *socks5.Datagram) error {
	log.Printf("new udp reqest: %s --> %s:%s", addr, d.DstAddr, d.DstPort)

	src := addr.String()
	var ch chan byte
	if s.LimitUDP {
		any, ok := s.AssociatedUDP.Get(src)
		if !ok {
			return fmt.Errorf("This udp address %s is not associated with tcp", src)
		}
		ch = any.(chan byte)
	}
	send := func(ue *socks5.UDPExchange, data []byte) error {
		select {
		case <-ch:
			return fmt.Errorf("This udp address %s is not associated with tcp", src)
		default:
			_, err := ue.RemoteConn.Write(data)
			if err != nil {
				return err
			}
			if socks5.Debug {
				log.Printf("Sent UDP data to remote. client: %#v server: %#v remote: %#v data: %#v\n", ue.ClientAddr.String(), ue.RemoteConn.LocalAddr().String(), ue.RemoteConn.RemoteAddr().String(), data)
			}
		}
		return nil
	}

	dst := d.Address()
	var ue *socks5.UDPExchange
	iue, ok := s.UDPExchanges.Get(src + dst)
	if ok {
		ue = iue.(*socks5.UDPExchange)
		return send(ue, d.Data)
	}

	if socks5.Debug {
		log.Printf("Call udp: %#v\n", dst)
	}
	var laddr string
	any, ok := s.UDPSrc.Get(src + dst)
	if ok {
		laddr = any.(string)
	}
	rc, err := socks5.DialUDP("udp", laddr, dst)
	if err != nil {
		if !strings.Contains(err.Error(), "address already in use") && !strings.Contains(err.Error(), "can't assign requested address") {
			return err
		}
		rc, err = socks5.DialUDP("udp", "", dst)
		if err != nil {
			return err
		}
		laddr = ""
	}
	if laddr == "" {
		s.UDPSrc.Set(src+dst, rc.LocalAddr().String(), -1)
	}
	ue = &socks5.UDPExchange{
		ClientAddr: addr,
		RemoteConn: rc,
	}
	if socks5.Debug {
		log.Printf("Created remote UDP conn for client. client: %#v server: %#v remote: %#v\n", addr.String(), ue.RemoteConn.LocalAddr().String(), d.Address())
	}
	if err := send(ue, d.Data); err != nil {
		ue.RemoteConn.Close()
		return err
	}
	s.UDPExchanges.Set(src+dst, ue, -1)
	go func(ue *socks5.UDPExchange, dst string) {
		defer func() {
			ue.RemoteConn.Close()
			s.UDPExchanges.Delete(ue.ClientAddr.String() + dst)
		}()
		var b [65507]byte
		for {
			select {
			case <-ch:
				if socks5.Debug {
					log.Printf("The tcp that udp address %s associated closed\n", ue.ClientAddr.String())
				}
				return
			default:
				if s.UDPTimeout != 0 {
					if err := ue.RemoteConn.SetDeadline(time.Now().Add(time.Duration(s.UDPTimeout) * time.Second)); err != nil {
						log.Println(err)
						return
					}
				}
				n, err := ue.RemoteConn.Read(b[:])
				if err != nil {
					return
				}
				if socks5.Debug {
					log.Printf("Got UDP data from remote. client: %#v server: %#v remote: %#v data: %#v\n", ue.ClientAddr.String(), ue.RemoteConn.LocalAddr().String(), ue.RemoteConn.RemoteAddr().String(), b[0:n])
				}
				a, addr, port, err := socks5.ParseAddress(dst)
				if err != nil {
					log.Println(err)
					return
				}
				if a == socks5.ATYPDomain {
					addr = addr[1:]
				}
				d1 := socks5.NewDatagram(a, addr, port, b[0:n])
				if _, err := s.UDPConn.WriteToUDP(d1.Bytes(), ue.ClientAddr); err != nil {
					return
				}
				if socks5.Debug {
					log.Printf("Sent Datagram. client: %#v server: %#v remote: %#v data: %#v %#v %#v %#v %#v %#v datagram address: %#v\n", ue.ClientAddr.String(), ue.RemoteConn.LocalAddr().String(), ue.RemoteConn.RemoteAddr().String(), d1.Rsv, d1.Frag, d1.Atyp, d1.DstAddr, d1.DstPort, d1.Data, d1.Address())
				}
			}
		}
	}(ue, dst)
	return nil
}
