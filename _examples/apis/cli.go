package apis

import (
	"fmt"

	"github.com/idrunk/dce-go/proto"
	"github.com/idrunk/dce-go/router"
)

func BindCli() {
	// Register a default command that responds with a welcome message when no specific command is provided.
	// Example usage: go run .
	proto.CliRouter.Push("", func(c *router.Context[*proto.CliProtocol]) {
		_, _ = c.WriteString("Welcome to DCE-GO!")
	})

	// Register a command to handle the "hello" API with an optional target parameter.
	// This command demonstrates handling of various command-line arguments, including:
	// - Positional arguments (e.g., "DCE-GO")
	// - Named arguments (e.g., "locate=zh-cn", "-locate zh-cn")
	// - Boolean flags (e.g., "-bool-true", "-bool-false=0", "--bool-true true")
	// - Array arguments (e.g., "-i=1", "-i 2", "-i 3", "-i=4")
	// Example usages:
	// - go run . hello DCE-GO
	// - go run . hello locate=zh-cn -locate zh-cn DCE-GO
	// - go run . hello DCE-GO -bool-true -bool-false=0 --bool-true true --locate=zh-cn
	// - go run . hello DCE-GO -i=1 -i 2 -i 3 -i=4
	proto.CliRouter.PushApi(router.Api{Path: "hello/{target?}", Responsive: false}, func(c *router.Context[*proto.CliProtocol]) {
		fmt.Printf("Hello %s!\n", c.Param("target"))
		fmt.Printf("Arg locate: %s\nArg -locate: %s\n", c.Rp.Arg("locate"), c.Rp.Arg("-locate"))
		fmt.Printf("Arg -bool-true: %t\nArg -bool-false: %t\nArg --bool-true: %t\n", c.Rp.Bool("-bool-true"), c.Rp.Bool("-bool-false"), c.Rp.Bool("--bool-true"))
		fmt.Printf("Array arg -i: %v\n", c.Rp.Args("-i"))
	})

	// Register a command to read input from a pipe or file.
	// This command reads the body of the input (e.g., from a pipe or file) and prints it.
	// Example usages:
	// - go run . read-pipe
	// - echo "Hello world!" | go run . read-pipe
	// - go run . read-pipe < go.mod
	proto.CliRouter.Push("read-pipe", func(c *router.Context[*proto.CliProtocol]) {
		body, _ := c.Rp.Body()
		fmt.Printf("parseBody from pipe:\n%sEOF\n", string(body))
	})
}
