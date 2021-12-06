package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/docopt/docopt-go"
)

const usage = `Generate files and run command.
Usage: reconf [-f -w <file> ...] [<command>...]

  <command>   Command to execute. If command is not given, reconf will
              just generate files and exit.

Options:
  -w, --render <file>  Generate <file> (if it does not exist) by rendering
                       template file named "<file>.template".
                       Optional format "<template file>:<render file>" allows to be more flexible
  -f, --force          Force generating files, overwriting existing ones.
  -h, --help           Show this usage message and exit.
`

const (
	version        = "v0.1"
	errorCode      = 120
	templateSuffix = ".template"
	templateSeparator = ":"
)

type Config struct {
	Files   []string `docopt:"--render"`
	Force   bool     `docopt:"--force"`
	Command []string `docopt:"<command>"`
}

func main() {
	parser := docopt.Parser{
		OptionsFirst: true,
	}
	opts, err := parser.ParseArgs(usage, os.Args[1:], version)
	if err != nil {
		panic(err)
	}

	var config Config
	if err := opts.Bind(&config); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(errorCode)
	}

	if err := run(config); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(errorCode)
	}
}

func run(config Config) error {
	envv := os.Environ()
	vars := map[string]interface{}{
		"env": mapEnviron(envv),
	}

	for _, filename := range config.Files {
		// Leave existing file as-is (unless forced).
		if _, err := os.Stat(filename); os.IsNotExist(err) || config.Force {
			if err := generate(filename, vars); err != nil {
				return err
			}
		}
	}

	// Just render templates and exit if command is not given.
	if len(config.Command) == 0 {
		return nil
	}

	// We just require the command path to be absolute if PATH is empty or
	// not set.
	paths := strings.Split(os.Getenv("PATH"), ":")

	return execvpe(config.Command[0], paths, config.Command, envv)
}

// Generates file by rendering corresponding template.
func generate(filename string, vars map[string]interface{}) error {

	tmplname := filename + templateSuffix
	if strings.Contains(filename, templateSeparator) {
		parts := strings.Split(filename, templateSeparator)
		tmplname = parts[0]
		filename = parts[1]
	}

	tmpl := template.New(tmplname)

	// Custom functions must be set before parsing template.
	tmpl.Funcs(templateFuncs)

	// ParseFiles() uses basename of the file as the name of the template. We
	// want the path of the file as-is.
	text, err := ioutil.ReadFile(tmplname)
	if err != nil {
		return err
	}

	if _, err := tmpl.Parse(string(text)); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := tmpl.Execute(file, vars); err != nil {
		os.Remove(filename)
		return err
	}

	return nil
}
