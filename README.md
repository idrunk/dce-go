[中文](README-zh.md) | English

---

**DCE-GO** is a powerful universal routing library that not only supports the HTTP protocol but also routes non-standard protocols such as CLI, WebSocket, TCP/UDP, and more. It adopts a modular design, divided into the following core modules based on functionality:

1. **Router Module**  
   As the core module of DCE, it defines APIs, contexts, and the router library, while providing interfaces for converters and routable protocols, ensuring flexibility and extensibility.

2. **Routable Protocol Module**  
   Encapsulates routable implementations of various common protocols, including HTTP, CLI, WebSocket, TCP, UDP, QUIC, etc., to meet diverse scenario requirements.

3. **Converter Module**  
   Built-in JSON and template converters support serialization and deserialization of serial data, as well as bidirectional conversion between transport objects and entity objects.

4. **Session Manager Module**  
   Defines interfaces for basic sessions, user sessions, connection sessions, and self-regenerating sessions, and provides implementation libraries for Redis and shared memory, facilitating rapid integration for developers.

5. **Utility Module**  
   Provides a series of practical tools to simplify the development process.

All features of DCE-GO come with detailed usage examples, located in the [_examples](_examples) directory. Its routing performance is comparable to Gin, and specific performance test reports can be viewed in the [ab test results](_examples/attachs/report/ab-test-result.txt), where port `2046` represents DCE's test results.

DCE-GO originates from [DCE-RUST](https://github.com/idrunk/dce-rust), and both are based on the core routing module of [DCE-PHP](https://github.com/idrunk/dce-php). DCE-PHP is a complete network programming framework that has ceased updates, with its core functionalities migrated to DCE-RUST and DCE-GO. Currently, DCE-GO has a newer feature version, and DCE-RUST will be synchronized with it in the future.

DCE is committed to building an efficient, open, and secure universal routing library, and welcomes community contributions to drive its development.

---

**TODO**:
- [ ] Optimize the JS version of the WebSocket routable protocol client.
- [ ] Upgrade the controller pre- and post-event interfaces to support binding with program interfaces.
- [ ] Enhance support for digital paths.
- [ ] Refactor elastic numeric functions into structural method styles.
- [ ] Investigate the possibility of supporting custom business attributes in routable protocols.
- [ ] Routable protocol support very large data packets such as file uploads.
- [ ] Upgrade the feature version of DCE-RUST.
- [ ] Improve the Golang client implementations for various protocols.
- [ ] Gradually replace AI-generated documentation with manually written documentation.

---

**Usage Example**

```golang
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"

	"go.drunkce.com/dce/converter"
	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/proto/flex"
	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/session"
	"go.drunkce.com/dce/util"
)

func main() {
	// go run main.go tcp start
	proto.CliRouter.Push("tcp/start/{address?}", func(c *proto.Cli) {
		bindServer()

		addr := c.ParamOr("address", ":2048")
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			panic(err.Error())
		}
		defer listener.Close()

		fmt.Printf("tcp server start at %s\n", addr)
		for {
			conn, err := listener.Accept()
			if err != nil {
				slog.Warn(fmt.Sprintf("accept error: %s", err))
				continue
			}
			go func(conn net.Conn) {
				defer conn.Close()
				// Connection sessions are used to store the connection information for sending message across hosts to clients in a distributed environment.
				shadow, err := session.NewShmSession[*Member](nil, session.DefaultTtlMinutes)
				if err != nil {
					slog.Warn(fmt.Sprintf("new session error: %s", err))
					return
				}
				shadow.Connect(conn.LocalAddr().String(), conn.RemoteAddr().String())
				defer shadow.Disconnect()
				for {
					if !flex.TcpRouter.Route(conn, map[string]any{"$shadowSession": shadow}) {
						break
					}
				}
			}(conn)
		}
	})
	bindClient()
	proto.CliRoute(1)
}

func bindServer() {
	flex.TcpRouter.Push("sign", func(c *flex.Tcp) {
		signInfo, ok := converter.JsonRawRequester[*flex.TcpProtocol, *Member](c).Parse()
		jsr := converter.JsonStatusResponser(c)
		if !ok {
			return
		}
		if (len(signInfo.Name) == 0 || len(signInfo.Password) == 0) && jsr.Fail("name or password is empty", 0) {
			return
		}
		member, ok := members[signInfo.Name]
		if !ok {
			// Notfound then auto register
			memberId++
			member = signInfo
			member.Id = memberId
			member.Role = 1
			members[member.Name] = member
		}
		if member.Password != signInfo.Password && jsr.Fail("password error", 0) {
			return
		}
		// Must be have a session obj after `BeforeController` event, so we no need to check nil
		se := c.Rp.Session().(*session.ShmSession[*Member])
		if err := se.Login(member, 0); err != nil && jsr.Fail(err.Error(), 0) {
			return
		}
		// Must be have a new session id after `UserSession.Login()`
		c.Rp.SetRespSid(se.Id())
		jc.Success(nil)
	})

	// Bind an api with Path: signer, roles: [1]
	flex.TcpRouter.PushApi(router.Path("signer").Append("roles", 1), func(c *flex.Tcp) {
		jc := converter.JsonResponser[*flex.TcpProtocol, *Member, *Signer](c)
		sess := c.Rp.Session().(*session.ShmSession[*Member])
		// Member info can be obtained here, so there is no need to check
		member, _ := sess.User()
		// Response the member, it can be convert to Signer struct automatically
		jc.Response(member)
	})

	flex.TcpRouter.SetEventHandler(func(c *flex.Tcp) error {
		shadow, _ := c.Rp.CtxData("$shadowSession")
		rs := shadow.(*session.ShmSession[*Member])
		cloned, err := rs.CloneForRequest(c.Rp.Sid())
		if err != nil {
			return err
		}
		se := cloned.(*session.ShmSession[*Member])
		if roles := util.MapSeqFrom[any, uint16](c.Api.ExtrasBy("roles")).Map(func(i any) uint16 {
			return uint16(i.(int))
		}).Collect(); len(roles) > 0 {
			// Roles configured means need to login
			if member, ok := se.User(); !ok {
				return util.Openly(401, "need to login")
			} else if !slices.Contains(roles, member.Role) {
				return util.Openly(403, "no permission")
			} else if newer, err := session.NewAutoRenew(se).TryRenew(); err != nil {
				return err
			} else if newer {
				// Logged session need to auto renew to enhance security
				c.Rp.SetRespSid(se.Id())
			}
		}
		c.Rp.SetSession(se)
		return nil
	}, nil)
}

func bindClient() {
	// go run main.go sign
	proto.CliRouter.Push("sign", func(c *proto.Cli) {
		reader := bufio.NewReader(os.Stdin)
		signInfo := Member{}
		fmt.Print("Enter username: ")
		username, _ := reader.ReadString('\n')
		signInfo.Name = strings.TrimSpace(username)
		fmt.Print("Enter password: ")
		password, _ := reader.ReadString('\n')
		signInfo.Password = strings.TrimSpace(password)
		reqBody, err := json.Marshal(signInfo)
		if err != nil && c.SetError(err) {
			return
		} else if resp := request(c, "sign", reqBody, ""); resp != nil {
			c.Rp.SetRespSid(resp.Sid)
			c.WriteString("Signed in successfully")
		}
	})

	// go run main.go signer $SESSION_ID
	proto.CliRouter.Push("signer/{sid?}", func(c *proto.Cli) {
		sid := c.Param("sid")
		if len(sid) == 0 {
			panic("Session ID is required")
		}
		if resp := request(c, "signer", nil, sid); resp == nil {
			c.SetError(util.Closed0("Request failed"))
		} else if resp.Code == 0 {
			var signer Signer
			if err := json.Unmarshal(resp.Body, &signer); err != nil && c.SetError(err) {
				return
			} else {
				// Just response the signer info if the session is logged in
				c.WriteString(fmt.Sprintf("Signer: %v", signer))
			}
		} else {
			c.SetError(util.Openly(int(resp.Code), resp.Message))
		}
	})
}

// It's a simple example, need to mapping request id and the response callback if the server is async
func request(c *proto.Cli, path string, reqBody []byte, sid string) *flex.Package {
	pkg := flex.NewPackage(path, reqBody, sid, -1)
	conn, _ := net.Dial("tcp", "127.0.0.1:"+c.Rp.ArgOr("port", "2048"))
	defer conn.Close()
	if _, err := conn.Write(pkg.Serialize()); err != nil && c.SetError(err) {
		return nil
	}
	resp, err := flex.PackageDeserialize(bufio.NewReader(conn))
	if err != nil && c.SetError(err) {
		return nil
	}
	return resp
}

var memberId uint64 = 0

var members map[string]*Member = make(map[string]*Member)

type Member struct {
	Id       uint64
	Role     uint16
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (m Member) Uid() uint64 {
	return m.Id
}

type Signer struct {
	Name string `json:"name"`
}

// Member entity converted to transfer object desensitization
func (m *Signer) From(member *Member) (*Signer, error) {
	m = util.NewStruct[*Signer]()
	m.Name = member.Name
	return m, nil
}
```