package apis

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/proto/flex"
	"go.drunkce.com/dce/router"
)

const alpn = "dce-quic-example"

func FlexQuicStart(c *proto.Cli) {
	flexQuicBind()
	port := c.Rp.ArgOr("-p", "2045")

	tlsCert, err := tls.LoadX509KeyPair("./attachs/cert/localhost.crt", "./attachs/cert/localhost.key")
	if err != nil {
		panic(err)
	}
	listener, err := quic.ListenAddr(":"+port, &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{alpn},
	}, nil)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Printf("FlexQuic server is started on port %s\n", port)
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		go func(conn quic.Connection) {
			for {
				if !flex.QuicRouter.Route(conn, nil) {
					break
				}
			}
		}(conn)
	}
}

func flexQuicBind() {
	// service apis

	// go run . quic localhost:2045 -- hello
	flex.QuicRouter.Push("hello", func(c *flex.Quic) {
		fmt.Printf("Api \"%s\": hello world\n", c.Api.Path)
		_, _ = c.WriteString("Hello world")
	})

	// go run . quic localhost:2045 -- echo "echo me"
	flex.QuicRouter.Push("echo/{param?}", func(c *flex.Quic) {
		param := c.Param("param")
		body, _ := c.Rp.Body()
		msg := fmt.Sprintf("path param data: %s\nbody data: %s", param, string(body))
		fmt.Println(msg)
		_, _ = c.WriteString(msg)
	})
}

func certPool() *x509.CertPool {
	certBytes, _ := os.ReadFile("./attachs/cert/localhost.crt")
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certBytes)
	return pool
}

func init() {
	proto.CliRouter.Push("quic/start", FlexQuicStart)

	// clients

	// go run . quic interactive localhost:2045
	// and then type in some param
	proto.CliRouter.Push("quic/interactive/{address}", func(c *proto.Cli) {
		addr := c.Param("address")
		tlsConf := &tls.Config{
			RootCAs:    certPool(),
			NextProtos: []string{alpn},
		}
		conn, err := quic.DialAddr(context.Background(), addr, tlsConf, &quic.Config{KeepAlivePeriod: 10 * time.Second})
		if err != nil {
			panic(err.Error())
		}
		defer conn.CloseWithError(0, "")

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Param: ")
			param, _ := reader.ReadString('\n')
			param = strings.TrimSpace(param)

			if strings.Compare("exit", param) == 0 {
				fmt.Println("exiting ...")
				break
			}
			path := "echo/" + param
			resp := quicRequest(conn, path)
			fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
		}
	})

	proto.CliRouter.Push("quic/{address}", func(c *proto.Cli) {
		addr := c.Param("address")
		tlsConf := &tls.Config{
			RootCAs:    certPool(),
			NextProtos: []string{alpn},
		}
		conn, err := quic.DialAddr(context.Background(), addr, tlsConf, nil)
		if err != nil {
			panic(err.Error())
		}
		defer conn.CloseWithError(0, "")

		passed := c.Rp.Passed
		if len(passed) == 0 {
			panic("passed args cannot be empty")
		}
		path := strings.Join(passed, router.MarkPathPartSeparator)
		resp := quicRequest(conn, path)
		fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
	})
}

func quicRequest(conn quic.Connection, path string) *flex.Package {
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		panic(err.Error())
	}
	defer stream.Close()
	hash := sha256.New()
	hash.Write([]byte(strconv.FormatUint(rand.Uint64(), 10)))
	content := []byte(fmt.Sprintf("Rand content「%X」", hash.Sum(nil)))
	if _, err := stream.Write(flex.NewPackage(path, content, "", -1).Serialize()); err != nil {
		panic(err.Error())
	}
	resp, err := flex.PackageDeserialize(bufio.NewReader(stream))
	if err != nil {
		println(err.Error())
	}
	return resp
}
