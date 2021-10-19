package main

import (
	"log"
	"os"

	"github.com/chyroc/draw-lark-sheet/internal"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "draw-lark-sheet",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "lark-app-id"},
			&cli.StringFlag{Name: "lark-app-secret"},
			&cli.StringFlag{Name: "lark-user-id"},
			&cli.StringFlag{Name: "image-path"},
		},
		Action: func(c *cli.Context) error {
			return internal.Run(internal.Request{
				LarkAppID:     c.String("lark-app-id"),
				LarkAppSecret: c.String("lark-app-secret"),
				LarkUserID:    c.String("lark-user-id"),
				ImagePath:     c.String("image-path"),
			})
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}
