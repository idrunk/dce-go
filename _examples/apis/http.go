package apis

import (
	"fmt"
	"net/http"

	"go.drunkce.com/dce/converter"
	"go.drunkce.com/dce/proto"
	"go.drunkce.com/dce/router"
	"go.drunkce.com/dce/util"
)

func init() {
	// go run . http start
	proto.CliRouter.Push("http/start", HttpStart)
}

func HttpStart(c *proto.Cli) {
	proto.HttpRouter.
		Get("{var1}", var1).
		Get("{var1}/var3/{var3?}", var3).
		Get("var4/{var4*}", var4).
		Get("var5/var5/{var5+}", var5).
		Get("var6/var6/{var6}/var6", var6).
		Get("session/{username?}", sessionApi).
		Get("hello", hello).
		Post("hello", helloPost).
		PushApi(router.Api{
			Path:     "home",
			Method:   proto.HttpGet,
			Omission: true,
		}, home).
		Raw().
		SetBefore("session/{username?}", func(ctx *router.Context[*proto.HttpProtocol]) error {
			if username := ctx.Param("username"); len(username) > 0 {
				ctx.Rp.SetCtxData("hello", "Inject from [BeforeController]")
			} else {
				return util.Openly(401, "Need to login")
			}
			return nil
		})

	port := c.Rp.ArgOr("-p", "2046")
	fmt.Printf("Http server is starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, http.HandlerFunc(proto.HttpRouter.Route)); err != nil {
		println(err.Error())
	}
}

// curl http://127.0.0.1:2046/Drunk
func var1(c *proto.Http) {
	user := c.Param("var1")
	c.Rp.Req.Header.Set("Content-Type", "text/xml")
	te := converter.TextTemplate[*proto.HttpProtocol, *Greeting](c, `<?xml version="1.0" encoding="UTF-8"?>
<greeting>
	<user>{{.User}}</user>
	<age>{{.Age}}</age>
	<welcome>{{.Welcome}}</welcome>
</greeting>`)
	te.Response(&Greeting{
		User:    user,
		Age:     0,
		Welcome: "Welcome to DCE-GO",
	})
}

// curl http://127.0.0.1:2046/var1/var3
// curl http://127.0.0.1:2046/var1/var3/var3
func var3(c *proto.Http) {
	var1 := c.Param("var1")
	var3 := c.Param("var3")
	fmt.Printf("var1: %s\nvar3: %s\n", var1, var3)
	_, _ = c.WriteString(fmt.Sprintf(`var1: %s(%d)<br/>var3: %s(%d)`, var1, len(var1), var3, len(var3)))
}

// curl http://127.0.0.1:2046/var4
// curl http://127.0.0.1:2046/var4/var4
func var4(c *proto.Http) {
	var4 := c.Params("var4")
	_, _ = c.WriteString(fmt.Sprintf(`var4: %s(%d)`, var4, len(var4)))
}

// curl http://127.0.0.1:2046/var5/var5/var5
// curl http://127.0.0.1:2046/var5/var5/var5/var5
func var5(c *proto.Http) {
	var5 := c.Params("var5")
	_, _ = c.WriteString(fmt.Sprintf(`var5: %s(%d)`, var5, len(var5)))
}

// curl http://127.0.0.1:2046/var6/var6/var6/var6
func var6(c *proto.Http) {
	var6 := c.Param("var6")
	_, _ = c.WriteString(fmt.Sprintf(`var6: %s(%d)`, var6, len(var6)))
}

// curl http://127.0.0.1:2046/session/dce
// curl http://127.0.0.1:2046/session/drunk
// curl -I http://127.0.0.1:2046/session
func sessionApi(c *proto.Http) {
	t := converter.StatusTemplate(c)
	if username := c.Param("username"); username == "dce" {
		msg, _ := c.Rp.CtxData("hello")
		fmt.Println(msg)
		_, _ = c.WriteString(msg.(string))
	} else {
		fmt.Println(c.Rp.Body())
		t.Fail("invalid manager", 403)
	}
}

// curl http://127.0.0.1:2046/hello
func hello(c *proto.Http) {
	_, _ = c.WriteString(`request via get`)
}

// curl -H "Content-Type: application/json" -d "{""user"":""Drunk"",""age"":18}" http://127.0.0.1:2046/hello
func helloPost(c *proto.Http) {
	var legalAge uint8 = 18
	jc := converter.JsonResponser[*proto.HttpProtocol, *Greeting, *GreetingResp](c)
	body, _ := converter.JsonRequester[*proto.HttpProtocol, *GreetingReq, *Greeting](c).Parse()
	fmt.Println(body)
	if body.Age >= legalAge {
		body.Welcome = fmt.Sprintf("Hello %s, welcome", body.User)
		jc.Response(body)
	} else {
		jc.Fail(fmt.Sprintf("Sorry, only service for over %d years old peoples", legalAge), 403)
	}
}

// curl http://127.0.0.1:2046/
func home(c *proto.Http) {
	jc := converter.JsonResponser[*proto.HttpProtocol, *Greeting, *GreetingResp](c)
	jc.Response(&Greeting{
		User:    "Dce",
		Age:     18,
		Welcome: "Welcome to Golang",
	})
}

type Greeting struct {
	User    string
	Age     uint8
	Welcome string
}

type GreetingReq struct {
	User string `json:"user"`
	Age  uint8  `json:"age"`
}

type GreetingResp struct {
	Welcome string `json:"welcome"`
}

func (gr *GreetingReq) Into() (*Greeting, error) {
	return &Greeting{
		User:    gr.User,
		Age:     gr.Age,
		Welcome: "",
	}, nil
}

func (gr *GreetingResp) From(g *Greeting) (*GreetingResp, error) {
	return &GreetingResp{
		Welcome: g.Welcome,
	}, nil
}
