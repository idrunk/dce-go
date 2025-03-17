package main

import (
	"go.drunkce.com/dce/_examples/apis"
	"go.drunkce.com/dce/proto"
)

func main() {
	apis.BindCli()
	proto.CliRoute(1)
}
