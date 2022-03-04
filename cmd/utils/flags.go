// Package utils contains internal helper functions for go-ethereum commands.
package utils

import (
	"flag"
	"io"
	"os"
	"os/user"
	"path"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/kardiachain/go-kardia/internal/flags"

	"gopkg.in/urfave/cli.v1"
)

func init() {
	cli.AppHelpTemplate = `{{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`
	cli.CommandHelpTemplate = flags.CommandHelpTemplate
	cli.HelpPrinter = printHelp
}

func printHelp(out io.Writer, templ string, data interface{}) {
	funcMap := template.FuncMap{"join": strings.Join}
	t := template.Must(template.New("help").Funcs(funcMap).Parse(templ))
	w := tabwriter.NewWriter(out, 38, 8, 2, ' ', 0)
	err := t.Execute(w, data)
	if err != nil {
		panic(err)
	}
	w.Flush()
}

// DirectoryString is a custom type which is registered in the flags library which cli uses for
// argument parsing. This allows us to expand Value to an absolute path when
// the argument is parsed
type DirectoryString string

func (s *DirectoryString) String() string {
	return string(*s)
}

func (s *DirectoryString) Set(value string) error {
	*s = DirectoryString(expandPath(value))
	return nil
}

// DirectoryFlag is the custom cli.Flag type which expand the received string to an absolute path.
// e.g. ~/.kardia -> /home/username/.kardia
type DirectoryFlag struct {
	Name   string
	Value  DirectoryString
	Usage  string
	EnvVar string
}

func (f DirectoryFlag) String() string {
	return cli.FlagStringer(f)
}

// Apply is called by cli library, grabs variable from environment (if in env)
// and adds variable to flag set for parsing.
func (f DirectoryFlag) Apply(set *flag.FlagSet) {
	eachName(f.Name, func(name string) {
		set.Var(&f.Value, f.Name, f.Usage)
	})
}

func (f DirectoryFlag) GetName() string {
	return f.Name
}

func (f *DirectoryFlag) Set(value string) {
	f.Value.Set(value)
}

func eachName(longName string, fn func(string)) {
	parts := strings.Split(longName, ",")
	for _, name := range parts {
		name = strings.Trim(name, " ")
		fn(name)
	}
}

// Expands a file path
// 1. replace tilde with users home dir
// 2. expands embedded environment variables
// 3. cleans the path, e.g. /a/b/../c -> /a/c
// Note, it has limitations, e.g. ~/username/tmp will not be expanded
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home := HomeDir(); home != "" {
			p = home + p[1:]
		}
	}
	return path.Clean(os.ExpandEnv(p))
}

func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.

var (
	// JS console related flags
	ExecFlag = cli.StringFlag{
		Name:  "exec",
		Usage: "Execute JavaScript statement",
	}
	PreloadJSFlag = cli.StringFlag{
		Name:  "preload",
		Usage: "Comma separated list of JavaScript files to preload into the console",
	}
	// ATM the url is left to the user and deployment to
	JSpathFlag = DirectoryFlag{
		Name:  "jspath",
		Usage: "JavaScript root path for `loadScript`",
		Value: DirectoryString("."),
	}
)

// MakeConsolePreloads retrieves the absolute paths for the console JavaScript
// scripts to preload before starting.
func MakeConsolePreloads(ctx *cli.Context) []string {
	// Skip preloading if there's nothing to preload
	if ctx.GlobalString(PreloadJSFlag.Name) == "" {
		return nil
	}
	// Otherwise resolve absolute paths and return them
	var preloads []string

	for _, file := range strings.Split(ctx.GlobalString(PreloadJSFlag.Name), ",") {
		preloads = append(preloads, strings.TrimSpace(file))
	}
	return preloads
}

func SplitTagsFlag(tagsFlag string) map[string]string {
	tags := strings.Split(tagsFlag, ",")
	tagsMap := map[string]string{}

	for _, t := range tags {
		if t != "" {
			kv := strings.Split(t, "=")

			if len(kv) == 2 {
				tagsMap[kv[0]] = kv[1]
			}
		}
	}

	return tagsMap
}

// MigrateFlags sets the global flag from a local flag when it's set.
// This is a temporary function used for migrating old command/flags to the
// new format.
//
// e.g. gkai account new --keystore /tmp/mykeystore --lightkdf
//
// is equivalent after calling this method with:
//
// gkai --keystore /tmp/mykeystore --lightkdf account new
//
// This allows the use of the existing configuration functionality.
// When all flags are migrated this function can be removed and the existing
// configuration functionality must be changed that is uses local flags
func MigrateFlags(action func(ctx *cli.Context) error) func(*cli.Context) error {
	return func(ctx *cli.Context) error {
		for _, name := range ctx.FlagNames() {
			if ctx.IsSet(name) {
				ctx.GlobalSet(name, ctx.String(name))
			}
		}
		return action(ctx)
	}
}
