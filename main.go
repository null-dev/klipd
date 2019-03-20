package main

import (
	"github.com/null-dev/klipd/klipd"
	"github.com/urfave/cli"
	"log"
	"os"
)

func main() {
	var ip, password string
	var port int

	app := cli.NewApp()
	app.Name = "klipd"
	app.Usage = "Universal clipboard sync daemon."
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:        "port",
			Value:       9559,
			Usage:       "The port to bind or connect to",
			Destination: &port,
		},
		cli.StringFlag{
			Name:        "password",
			Usage:       "The password to encrypt communications with",
			Destination: &password,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "client",
			Aliases:   []string{"c"},
			Usage:     "Run the daemon in 'client' mode.",
			ArgsUsage: "--ip <server ip> --password <password>",
			Action: func(c *cli.Context) {
				if ip == "" {
					log.Fatal("No IP specified!")
					return
				}

				klipd.StartClient(password, ip, port)
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "ip",
					Usage:       "The IP to connect to",
					Destination: &ip,
				},
			},
		},
		{
			Name:      "server",
			Aliases:   []string{"s"},
			Usage:     "Run the daemon in 'server' mode.",
			ArgsUsage: "--password <password>",
			Action: func(c *cli.Context) {
				klipd.StartServer(password, ip, port)
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "ip",
					Value:       "0.0.0.0",
					Usage:       "The IP to bind to",
					Destination: &ip,
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
