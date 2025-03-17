中文 | [English](README.md)

---

**DCE-GO** 是一个功能强大的通用路由库，不仅支持 HTTP 协议，还能路由 CLI、WebSocket、TCP/UDP 等非标准协议。它采用模块化设计，按功能划分为以下核心模块：

1. **路由器模块**  
   作为 DCE 的核心模块，定义了 API、上下文及路由器库，同时提供了转换器、可路由协议等接口，确保灵活性和扩展性。

2. **可路由协议模块**  
   封装了多种常见协议的可路由实现，包括 HTTP、CLI、WebSocket、TCP、UDP、QUIC 等，满足多样化场景需求。

3. **转换器模块**  
   内置 JSON 和模板转换器，支持串行数据的序列化与反序列化，以及传输对象与实体对象的双向转换。

4. **会话管理器模块**  
   定义了基础会话、用户会话、连接会话及自重生会话接口，并提供了 Redis 和共享内存的实现类库，方便开发者快速集成。

5. **工具模块**  
   提供了一系列实用工具，简化开发流程。

DCE-GO 的所有功能特性均配有详细用例，位于 [_examples](_examples) 目录下。其路由性能与 Gin 相当，具体性能测试报告可查看 [ab 测试结果](_examples/attachs/report/ab-test-result.txt)，其中端口 `2046` 为 DCE 的测试结果。

DCE-GO 源自 [DCE-RUST](https://github.com/idrunk/dce-rust)，而两者均基于 [DCE-PHP](https://github.com/idrunk/dce-php) 的核心路由模块升级而来。DCE-PHP 是一个完整的网络编程框架，现已停止更新，其核心功能已迁移至 DCE-RUST 和 DCE-GO。目前，DCE-GO 的功能版本较新，未来 DCE-RUST 将与之同步。

DCE 致力于打造一个高效、开放、安全的通用路由库，欢迎社区贡献，共同推动其发展。

---

**TODO**：
- [ ] 优化 JS 版 WebSocket 可路由协议客户端。
- [ ] 升级控制器前后置事件接口，支持与程序接口绑定。
- [ ] 完善数字路径支持。
- [ ] 调整弹性数字函数为结构方法式。
- [ ] 研究可路由协议中支持自定义业务属性的可能性。
- [ ] 支持文件上传等超大数据包的可路由协议。
- [ ] 升级 DCE-RUST 功能版本。
- [ ] 完善各协议的 Golang 客户端实现。
- [ ] 逐步替换 AI 生成的文档为人工编写文档。

---

**使用示例**

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
		jsr.Success(nil)
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