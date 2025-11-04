package shared

import (
	"github.com/urfave/cli/v3"
)

func NewDirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "dir",
		Usage: "Working directory which contains the .bumper directory",
	}
}

func DirFlag(c *cli.Command) string {
	return c.String("dir")
}
