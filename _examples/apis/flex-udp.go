package apis

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"log"
	"log/slog"
	"math/rand/v2"
	"net"
	"strconv"
	"strings"

	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/proto/flex"
	"go.drunkce.com/dce/router"
)

func FlexUdpStart(c *proto.Cli) {
	flexUdpBind()
	port := c.Rp.ArgOr("-p", "2049")
	localAddr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:"+port)
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("FlexUdp server is started on port %s\n", port)
	for {
		var pkg = make([]byte, 8192)
		n, addr, err := conn.ReadFromUDP(pkg)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		go func(n int, addr *net.UDPAddr, pkg []byte) {
			flex.UdpRoute(conn, pkg[:n], addr, nil)
		}(n, addr, pkg)
	}
}

func flexUdpBind() {
	// go run . udp 127.0.0.1:2049 -- hello
	flex.UdpRouter.Push("hello", func(c *flex.Udp) {
		fmt.Printf("Api \"%s\": hello world\n", c.Api.Path)
		_, _ = c.WriteString("Hello world")
	})

	// go run . udp 127.0.0.1:2049 -- echo "echo me"
	flex.UdpRouter.Push("echo/{param?}", func(c *flex.Udp) {
		param := c.Param("param")
		body, _ := c.Rp.Body()
		_, _ = c.WriteString(fmt.Sprintf("path param data: %s\nbody data: %s", param, string(body)))
	})
}

func init() {
	proto.CliRouter.Push("udp/start", FlexUdpStart)

	proto.CliRouter.Push("udp/{address}", func(c *proto.Cli) {
		addr := c.Param("address")
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			panic("not a valid address")
		}
		dial, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			panic(err.Error())
		}
		defer dial.Close()
		passed := c.Rp.Passed
		if len(passed) == 0 {
			panic("passed args cannot be empty")
		}
		path := strings.Join(passed, router.MarkPathPartSeparator)
		hash := sha256.New()
		hash.Write([]byte(strconv.FormatUint(rand.Uint64(), 10)))
		content := []byte(fmt.Sprintf("Rand content「%X」", hash.Sum(nil)))
		if _, err := dial.Write(flex.NewPackage(path, content, "", -1).Serialize()); err != nil {
			panic(err.Error())
		}
		resp, err := flex.PackageDeserialize(bufio.NewReader(dial))
		if err != nil {
			println(err.Error())
		}
		fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
	})
}
