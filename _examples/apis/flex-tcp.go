package apis

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/proto/flex"
	"github.com/idrunk/dce-go/router"
	"log"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
)

func FlexTcpStart(c *proto.Cli) {
	flexTcpBind()
	port := c.Rp.ArgOr("-p", "2048")

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Printf("FlexTcp server is started on port %s\n", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn(err.Error())
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			for {
				if !flex.TcpRouter.Route(conn, nil) {
					break
				}
			}
		}(conn)
	}
}

func flexTcpBind() {
	// service apis

	// go run . tcp 127.0.0.1:2048 -- hello
	flex.TcpRouter.Push("hello", func(c *flex.Tcp) {
		fmt.Printf("Api \"%s\": hello world\n", c.Api.Path)
		_, _ = c.WriteString("Hello world")
	})

	// go run . tcp 127.0.0.1:2048 -- echo "echo me"
	flex.TcpRouter.Push("echo/{param?}", func(c *flex.Tcp) {
		param := c.Param("param")
		body, _ := c.Rp.Body()
		_, _ = c.WriteString(fmt.Sprintf("path param data: %s\nbody data: %s", param, string(body)))
	})
}

func init() {
	proto.CliRouter.Push("tcp/start", FlexTcpStart)

	// clients

	// go run . tcp interactive 127.0.0.1:2048
	// and then type in some param
	proto.CliRouter.Push("tcp/interactive/{address}", func(c *proto.Cli) {
		addr := c.Param("address")
		dial, err := net.Dial("tcp", addr)
		if err != nil {
			panic(err.Error())
		}
		defer dial.Close()
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
			resp := tcpRequest(dial, path)
			fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
		}
	})

	proto.CliRouter.Push("tcp/{address}", func(c *proto.Cli) {
		addr := c.Param("address")
		if len(addr) == 0 {
			panic("not a valid address")
		}
		dial, err := net.Dial("tcp", addr)
		if err != nil {
			panic(err.Error())
		}
		defer dial.Close()
		passed := c.Rp.Passed
		if len(passed) == 0 {
			panic("passed args cannot be empty")
		}
		path := strings.Join(passed, router.MarkPathPartSeparator)
		resp := tcpRequest(dial, path)
		fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
	})
}

func tcpRequest(dial net.Conn, path string) *flex.Package {
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
	return resp
}
