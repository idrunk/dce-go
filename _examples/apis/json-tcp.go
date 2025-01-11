package apis

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/proto/flex"
	"github.com/idrunk/dce-go/proto/json"
	"github.com/idrunk/dce-go/router"
	"log"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
)

func JsonTcpStart(c *proto.Cli) {
	jsonTcpBind()
	port := c.Rp.ArgOr("-p", "3048")

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Printf("JsonTcp server is started on port %s\n", port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn(err.Error())
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			for {
				if !json.TcpRouter.Route(conn, nil) {
					break
				}
			}
		}(conn)
	}
}

func jsonTcpBind() {
	// service apis

	// go run . json-tcp 127.0.0.1:3048 -- hello
	json.TcpRouter.Push("hello", func(c *json.Tcp) {
		fmt.Printf("Api \"%s\": hello world\n", c.Api.Path)
		_, _ = c.WriteString("Hello world")
	})

	// go run . json-tcp 127.0.0.1:3048 -- echo "echo me"
	json.TcpRouter.Push("echo/{param?}", func(c *json.Tcp) {
		param := c.Param("param")
		body, _ := c.Rp.Body()
		_, _ = c.WriteString(fmt.Sprintf("path param data: %s\nbody data: %s", param, string(body)))
	})
}

func init() {
	// go run . json-tcp start
	proto.CliRouter.Push("json-tcp/start", JsonTcpStart)

	// clients

	// go run . json-tcp interactive 127.0.0.1:3048
	// and then type in some param
	proto.CliRouter.Push("json-tcp/interactive/{address}", func(c *proto.Cli) {
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
			resp := jsonTcpRequest(dial, path)
			fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
		}
	})

	proto.CliRouter.Push("json-tcp/{address}", func(c *proto.Cli) {
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
		resp := jsonTcpRequest(dial, path)
		fmt.Printf("Got resp:\n%s(%d)\n", resp.Body, len(resp.Body))
	})
}

func jsonTcpRequest(dial net.Conn, path string) *json.Package {
	hash := sha256.New()
	hash.Write([]byte(strconv.FormatUint(rand.Uint64(), 10)))
	content := []byte(fmt.Sprintf("Rand content「%X」", hash.Sum(nil)))
	if _, err := dial.Write(flex.StreamPack(json.NewPackage(path, content, "", -1).Serialize())); err != nil {
		panic(err.Error())
	}
	result, err := flex.StreamRead(dial)
	if err != nil {
		println(err.Error())
	}
	resp, err := json.PackageDeserialize(result)
	if err != nil {
		println(err.Error())
	}
	return resp
}
