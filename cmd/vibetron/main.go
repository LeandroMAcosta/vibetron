package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/LeandroMAcosta/vibetron/internal/net"
	"github.com/LeandroMAcosta/vibetron/internal/ui"
)

func main() {
	var opts ui.Options
	flag.StringVar(&opts.Name, "name", defaultName(), "tu nombre en la sala")
	flag.BoolVar(&opts.Host, "host", false, "crear una sala y mostrar el código")
	flag.BoolVar(&opts.Solo, "solo", false, "jugar contra bots")
	flag.BoolVar(&opts.Demo, "demo", false, "modo demo: todos son bots")
	flag.StringVar(&opts.Join, "join", "", "unirse a una sala con su código")
	flag.Parse()
	if r := []rune(opts.Name); len(r) > net.MaxName {
		opts.Name = string(r[:net.MaxName])
	}
	if err := ui.Run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "vibetron:", err)
		os.Exit(1)
	}
}

func defaultName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return strings.ToUpper(u.Username)
	}
	return "JUGADOR"
}
