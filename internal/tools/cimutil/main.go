package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const cimPath = "cimtest.cim"

func main() {
	app := cli.NewApp()
	app.Name = "cimutil"
	app.Usage = "Control CimFS"

	app.Commands = []cli.Command{
		{
			Name:  "mount",
			Usage: "Mount an image",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path",
					Usage: "Path to the CimFS image to operate on",
				},
			},
			Action: func(c *cli.Context) error {
				g, err := cimfs.MountImage(c.String("path"))
				if err != nil {
					return err
				}
				fmt.Println(g)
				return nil
			},
		},
		{
			Name:  "unmount",
			Usage: "Unmount an image",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "id",
					Usage: "Volume ID of the image to unmount",
				},
			},
			Action: func(c *cli.Context) error {
				g, err := guid.FromString(c.String("id"))
				if err != nil {
					return err
				}
				return cimfs.UnmountImage(g)
			},
		},
		{
			Name:  "add",
			Usage: "Add a file to an image",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path",
					Usage: "Path to the CimFS image to operate on",
				},
				cli.StringFlag{
					Name:  "src-path",
					Usage: "Path to the file to add",
				},
				cli.StringFlag{
					Name:  "dst-path",
					Usage: "Path where the file is added in the CimFS image",
				},
			},
			Action: func(c *cli.Context) error {
				path := c.String("path")
				src := c.String("src-path")
				dst := c.String("dst-path")

				img, err := cimfs.Open(path)
				if err != nil {
					return err
				}
				defer img.Close(path)

				stat, err := os.Stat(src)
				if err != nil {
					return err
				}

				info := &cimfs.FileInfo{
					Size: stat.Size(),
				}

				if err := img.AddFile(dst, info); err != nil {
					return err
				}

				f, err := os.Open(src)
				if err != nil {
					return err
				}

				if _, err := io.Copy(img, f); err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name:  "rm",
			Usage: "Remove a file from an image",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path",
					Usage: "Path to the CimFS image to operate on",
				},
				cli.StringFlag{
					Name:  "file-path",
					Usage: "Path to the file to remove",
				},
			},
			Action: func(c *cli.Context) error {
				path := c.String("path")

				img, err := cimfs.Open(path)
				if err != nil {
					return err
				}
				defer img.Close(path)

				if err := img.RemoveFile(c.String("file-path")); err != nil {
					return err
				}

				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
