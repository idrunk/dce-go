package main

import (
	"github.com/idrunk/dce-go/_examples/apis"
	"github.com/idrunk/dce-go/proto"
)

func main() {
	apis.BindCli()
	proto.CliRoute(1)
}
