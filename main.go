package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:  "gen",
				Usage: "generate models and mongodb api",
				Action: func(cCtx *cli.Context) error {
					fmt.Println(cCtx.Args())
					fmt.Println("added task: ", cCtx.String("add"))
					return nil
				},
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:     "json",
						Aliases:  []string{"j"},
						Required: true,
						Usage:    "path to the *.json file which contains model templates",
					},
					&cli.PathFlag{
						Name:    "dir",
						Aliases: []string{"d"},
						Value:   "./",
						Usage:   "generated code will be located in this dir",
					},
					&cli.StringSliceFlag{
						Name:    "add",
						Aliases: []string{"a"},
						Usage:   "add global field in models, format: <name>:<type>",
					},
				},
			},
			{
				Name:  "types",
				Usage: "list available global field types mapping from reed to Go",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("\nAvailable global field types:\t")
					fmt.Println("--------------------------\t")
					fmt.Println("reed\t", "\t", "Go")
					fmt.Println("--------------------------\t")
					fmt.Println("int\t", "--->\t", "int64")
					fmt.Println("float\t", "--->\t", "float64")
					fmt.Println("time\t", "--->\t", "time.Time")
					fmt.Println("string\t", "--->\t", "string")
					fmt.Println("bool\t", "--->\t", "bool")
					fmt.Println("--------------------------\t")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
