package proto

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.drunkce.com/dce/router"
)

const (
	MarkPassedSeparator = "--"
	MarkAssignment      = "="
	MarkArgPrefix       = "-"
)

const (
	ArgTypeAssignExpr = iota + 1
	ArgTypePrefixName
	ArgTypePath
	ArgTypePassedSeparator
)

type Cli = router.Context[*CliProtocol]

type CliProtocol struct {
	router.Meta[[]string]
	Passed []string
	path   string
	args   cliArgs
	body   io.Reader
}

func CliRoute(base int) {
	c := &CliProtocol{Meta: router.NewMeta(os.Args[base:], nil, true)}
	c.parse()
	ctx := router.NewContext(c)
	CliRouter.Route(ctx)
	c.TryPrintErr()
	if ctx.Api != nil && ctx.Api.Responsive {
		if sid := c.RespSid(); sid != "" {
			if _, err := c.WriteString("\n\nNew sid: " + sid); err != nil {
				println(err)
			}
		}
		resp := string(c.ClearBuffer()[:])
		fmt.Println(resp)
	}
}

func parseType(arg string) (int, string, string) {
	if parts := strings.SplitN(arg, MarkAssignment, 2); len(parts) == 2 {
		return ArgTypeAssignExpr, parts[0], parts[1]
	} else if strings.HasPrefix(arg, MarkArgPrefix) {
		if arg == MarkPassedSeparator {
			return ArgTypePassedSeparator, "", ""
		}
		return ArgTypePrefixName, "", ""
	} else {
		return ArgTypePath, "", ""
	}
}

func (c *CliProtocol) parse() {
	var paths []string
	c.args = cliArgs{map[string]string{}, map[string][]string{}}
Loop:
	for i := 0; i < len(c.Req); i++ {
		arg := c.Req[i]
		switch ty, left, right := parseType(arg); ty {
		case ArgTypeAssignExpr:
			c.args.setValue(left, right)
		case ArgTypePrefixName:
			if len(c.Req) > i {
				if ty, _, _ = parseType(c.Req[i+1]); ty == ArgTypePath {
					i++
					c.args.setValue(arg, c.Req[i])
					continue
				}
			}
			c.args.setValue(arg, "true")
		case ArgTypePassedSeparator:
			c.Passed = c.Req[i+1:]
			break Loop
		default:
			paths = append(paths, arg)
		}
	}
	c.path = strings.Join(paths, router.MarkPathPartSeparator)
	c.parseBody()
}

func (c *CliProtocol) parseBody() {
	buffer := bytes.NewBuffer(nil)
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		read := make(chan byte, 1)
		go func() {
			var buf [1]byte
			if _, err := os.Stdin.Read(buf[:]); err == nil {
				read <- buf[0]
			}
		}()
		select {
		case first := <-read:
			buffer.WriteByte(first)
			if bts, err := io.ReadAll(os.Stdin); err == nil {
				buffer.Write(bts)
			} else {
				slog.Debug(err.Error())
			}
		case <-time.After(32 * time.Millisecond):
			// sometimes (like run with goland), will enter this outer "if" logic incorrectly, and the program will be
			// blocked at the "Stdin.Read" forever, so we use this timeout logic to cancel the reading action
			_ = os.Stdin.Close()
		}
	}
	c.body = buffer
}

func (c *CliProtocol) Path() string {
	return c.path
}

func (c *CliProtocol) Body() ([]byte, error) {
	return io.ReadAll(c.body)
}

func (c *CliProtocol) Bool(key string) bool {
	val := c.args.scalars[key]
	return val == "true" || val == "1"
}

func (c *CliProtocol) Arg(key string) string {
	return c.args.scalars[key]
}

func (c *CliProtocol) ArgOr(key string, def string) string {
	if v, ok := c.args.scalars[key]; ok {
		return v
	}
	return def
}

func (c *CliProtocol) Args(key string) []string {
	return c.args.vectors[key]
}

func (c *CliProtocol) Scalars() map[string]string {
	return c.args.scalars
}

func (c *CliProtocol) Vectors() map[string][]string {
	return c.args.vectors
}

type cliArgs struct {
	scalars map[string]string
	vectors map[string][]string
}

func (c *cliArgs) setValue(key string, value string) {
	if vector, ok := c.vectors[key]; ok {
		delete(c.scalars, key)
		c.vectors[key] = append(vector, value)
	} else {
		c.scalars[key] = value
		c.vectors[key] = []string{value}
	}
}

var CliRouter *router.Router[*CliProtocol]

func init() {
	CliRouter = router.ProtoRouter[*CliProtocol]("cli")
}
