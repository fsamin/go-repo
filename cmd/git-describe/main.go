package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/fsamin/go-repo"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "git-describe",
		Usage: "git describe on steroïds",
		Args:  true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "long",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "long-semver",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "match",
				Value: "v[0-9]*",
			},
			&cli.StringFlag{
				Name:  "dirty-mark",
				Value: "-dirty",
			},
			&cli.BoolFlag{
				Name:  "dirty-semver",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "required-annotated",
				Value: false,
			},
		},
		Action: func(ctx *cli.Context) error {
			var path = "."
			if ctx.Args().Len() == 1 {
				path = ctx.Args().Get(0)
			}

			r, err := repo.New(context.Background(), path)
			if err != nil {
				return err
			}

			var opts = &repo.DescribeOpt{
				Long:             ctx.Bool("long"),
				LongSemver:       ctx.Bool("long-semver"),
				Match:            ctx.String("match"),
				DirtySemver:      ctx.Bool("dirty-semver"),
				DirtyMark:        ctx.String("dirty-mark"),
				RequireAnnotated: ctx.Bool("required-annotated"),
			}

			d, err := r.Describe(context.Background(), opts)
			if err != nil {
				return err
			}

			btes, _ := json.MarshalIndent(d, "  ", "  ")
			fmt.Println(string(btes))

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
