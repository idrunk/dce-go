package apis

import (
	"fmt"
	"net/http"

	"github.com/idrunk/dce-go/proto"
	"github.com/quic-go/quic-go/http3"
)

func init() {
	proto.CliRouter.
		// go run . http2 start
		Push("http2/start", Http2Start).
		// go run . http3 start
		Push("http3/start", Http3Start)
}

func Http2Start(c *proto.Cli) {
	// curl --insecure https://localhost:2043/hello
	proto.HttpRouter.Get("hello", func(h *proto.Http) {
		_, _ = h.WriteString("hello world")
	})

	// curl -v --insecure https://localhost:2043/
	// curl --insecure https://localhost:2043/DCE
	proto.HttpRouter.Get("{target?}", func(h *proto.Http) {
		_, _ = h.WriteString("hello " + h.Param("target"))
	})

	port := c.Rp.ArgOr("-p", "2043")
	fmt.Printf("Http2 server is starting on port %s\n", port)
	if err := http.ListenAndServeTLS(":"+port, "./attachs/cert/localhost.crt", "./attachs/cert/localhost.key", http.HandlerFunc(proto.HttpRouter.Route)); err != nil {
		println(err.Error())
	}
}

func Http3Start(c *proto.Cli) {
	// curl --http3 --insecure https://localhost:2044/hello
	proto.HttpRouter.Get("hello", func(h *proto.Http) {
		_, _ = h.WriteString("hello world")
	})

	// curl -v --http3 --insecure https://localhost:2044/
	// curl --http3 --insecure https://localhost:2044/DCE
	proto.HttpRouter.Get("{target?}", func(h *proto.Http) {
		_, _ = h.WriteString("hello " + h.Param("target"))
	})

	port := c.Rp.ArgOr("-p", "2044")
	fmt.Printf("Http3 server is starting on port %s\n", port)
	if err := http3.ListenAndServeQUIC(":"+port, "./attachs/cert/localhost.crt", "./attachs/cert/localhost.key", http.HandlerFunc(proto.HttpRouter.Route)); err != nil {
		println(err.Error())
	}
}
