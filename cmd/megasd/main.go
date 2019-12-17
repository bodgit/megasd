package main

import (
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/bodgit/megasd"
	"github.com/bodgit/megasd/image"
	"github.com/urfave/cli/v2"
)

const defaultDB = "megasd.db"

func init() {
	cli.VersionFlag = &cli.BoolFlag{
		Name:  "version, V",
		Usage: "print the version",
	}
}

func main() {
	app := cli.NewApp()

	app.Name = "megasd"
	app.Usage = "Terraonion MegaSD management utility"
	app.Version = "1.0.0"

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "db",
			EnvVars: []string{"MEGASD_DB"},
			Value:   filepath.Join(cwd, defaultDB),
			Usage:   "path to database",
		},
		&cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "increase verbosity",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:        "encode",
			Usage:       "Encode and convert a PNG image to MegaSD format",
			Description: "The PNG is read from the standard input and the converted image is written to standard output",
			Action: func(c *cli.Context) error {
				m, err := png.Decode(os.Stdin)
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				if err := image.Encode(os.Stdout, m); err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:        "decode",
			Usage:       "Decode a MegaSD image back to PNG format",
			Description: "The image is read from the standard input and the PNG is written to standard output",
			Action: func(c *cli.Context) error {
				m, err := image.Decode(os.Stdin)
				if err != nil {
					return cli.NewExitError(err, 1)
				}

				if err := png.Encode(os.Stdout, m); err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:        "import",
			Usage:       "Import XML and screenshots from C# tool",
			Description: "",
			ArgsUsage:   "FILE",
			Action: func(c *cli.Context) error {
				if c.NArg() < 1 {
					cli.ShowCommandHelpAndExit(c, c.Command.FullName(), 1)
				}

				logger := log.New(ioutil.Discard, "", 0)
				if c.Bool("verbose") {
					logger.SetOutput(os.Stderr)
				}

				m, err := megasd.New(c.String("db"), logger)
				if err != nil {
					return cli.NewExitError(err, 1)
				}
				defer m.Close()

				if err := m.ImportXML(c.Args().First()); err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
		{
			Name:        "scan",
			Usage:       "Scan filesystem and generate metadata",
			Description: "",
			ArgsUsage:   "DIRECTORY",
			Action: func(c *cli.Context) error {
				if c.NArg() < 1 {
					cli.ShowCommandHelpAndExit(c, c.Command.FullName(), 1)
				}

				logger := log.New(ioutil.Discard, "", 0)
				if c.Bool("verbose") {
					logger.SetOutput(os.Stderr)
				}

				m, err := megasd.New(c.String("db"), logger)
				if err != nil {
					return cli.NewExitError(err, 1)
				}
				defer m.Close()

				if err := m.Scan(c.Args().First()); err != nil {
					return cli.NewExitError(err, 1)
				}

				return nil
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
