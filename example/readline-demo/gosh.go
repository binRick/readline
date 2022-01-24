package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"plugin"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/vladimirvivien/gosh/api"
)

var (
	reCmd = regexp.MustCompile(`\S+`)
	shell = New()
)

type Goshell struct {
	ctx        context.Context
	pluginsDir string
	commands   map[string]api.Command
	closed     chan struct{}
}

func reloadShell() {
	shell = New()
}

// New returns a new shell
func New() *Goshell {
	return &Goshell{
		pluginsDir: api.PluginsDir,
		commands:   make(map[string]api.Command),
		closed:     make(chan struct{}),
	}
}

// Init initializes the shell with the given context
func (gosh *Goshell) Init(ctx context.Context) error {
	gosh.ctx = ctx
	return gosh.loadCommands()
}

func (gosh *Goshell) buildFiles() (time.Duration, error) {
	var wg sync.WaitGroup
	st := time.Now()
	for _, cmd := range shell.buildFileCommands() {
		wg.Add(1)
		func() {
			out, err := exec.Command(`sh`, `-c`, cmd).Output()
			if err != nil {
				if len(out) > 0 {
					fmt.Fprintf(os.Stderr, `%s`, out)
				}
				log.Fatal(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	reloadShell()
	return time.Since(st), nil
}

func (gosh *Goshell) buildFileCommands() (cmds []string) {
	for _, gf := range shell.listFiles() {
		cmd := fmt.Sprintf(`go build -buildmode=plugin -o "%s/%s_command.so" "%s/%s"`,
			gosh.pluginsDir,
			strings.Replace(strings.Split(gf, `.`)[0], `cmd`, ``, -1),
			gosh.pluginsDir,
			gf,
		)
		cmds = append(cmds, cmd)
	}
	return unique(cmds)
}

func unique(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func (gosh *Goshell) listFiles() []string {
	gosh_files, err := listGoshFiles(gosh.pluginsDir, `.*.go`)
	if err != nil {
		panic(err)
	}
	ps := []string{}
	for _, gosh_file := range gosh_files {
		ps = append(ps, gosh_file.Name())
	}
	return ps
}
func (gosh *Goshell) listPlugins() []string {
	plugins, err := listGoshFiles(gosh.pluginsDir, `.*_command.so`)
	if err != nil {
		panic(err)
	}
	ps := []string{}
	for _, cmdPlugin := range plugins {
		plug, err := plugin.Open(path.Join(gosh.pluginsDir, cmdPlugin.Name()))
		if err != nil {
			fmt.Printf("failed to open plugin %s: %v\n", cmdPlugin.Name(), err)
			continue
		}
		if false {
			fmt.Println(plug)
		}
		ps = append(ps, cmdPlugin.Name())

	}
	return ps
}

func (gosh *Goshell) loadCommands() error {
	if _, err := os.Stat(gosh.pluginsDir); err != nil {
		return err
	}

	plugins, err := listGoshFiles(gosh.pluginsDir, `.*_command.so`)
	if err != nil {
		return err
	}

	for _, cmdPlugin := range plugins {
		plug, err := plugin.Open(path.Join(gosh.pluginsDir, cmdPlugin.Name()))
		if err != nil {
			fmt.Printf("failed to open plugin %s: %v\n", cmdPlugin.Name(), err)
			continue
		}
		cmdSymbol, err := plug.Lookup(api.CmdSymbolName)
		if err != nil {
			fmt.Printf("plugin %s does not export symbol \"%s\"\n",
				cmdPlugin.Name(), api.CmdSymbolName)
			continue
		}
		commands, ok := cmdSymbol.(api.Commands)
		if !ok {
			fmt.Printf("Symbol %s (from %s) does not implement Commands interface\n",
				api.CmdSymbolName, cmdPlugin.Name())
			continue
		}
		if err := commands.Init(gosh.ctx); err != nil {
			fmt.Printf("%s initialization failed: %v\n", cmdPlugin.Name(), err)
			continue
		}
		for name, cmd := range commands.Registry() {
			gosh.commands[name] = cmd
			if false {
				fmt.Fprintf(os.Stderr, "Cmd: %s\n", name)
			}
		}
		gosh.ctx = context.WithValue(gosh.ctx, "gosh.commands", gosh.commands)
	}
	return nil
}

// Open opens the shell for the given reader
func (gosh *Goshell) Open(r *bufio.Reader) {
	loopCtx := gosh.ctx
	line := make(chan string)
	for {
		// start a goroutine to get input from the user
		go func(ctx context.Context, input chan<- string) {
			for {
				// TODO: future enhancement is to capture input key by key
				// to give command granular notification of key events.
				// This could be used to implement command autocompletion.
				fmt.Fprintf(ctx.Value("gosh.stdout").(io.Writer), "%s ", api.GetPrompt(loopCtx))
				line, err := r.ReadString('\n')
				if err != nil {
					fmt.Fprintf(ctx.Value("gosh.stderr").(io.Writer), "%v\n", err)
					continue
				}

				input <- line
				return
			}
		}(loopCtx, line)

		// wait for input or cancel
		select {
		case <-gosh.ctx.Done():
			close(gosh.closed)
			return
		case input := <-line:
			var err error
			loopCtx, err = gosh.handle(loopCtx, input)
			if err != nil {
				fmt.Fprintf(loopCtx.Value("gosh.stderr").(io.Writer), "%v\n", err)
			}
		}
	}
}

// Closed returns a channel that closes when the shell has closed
func (gosh *Goshell) Closed() <-chan struct{} {
	return gosh.closed
}

func (gosh *Goshell) handle(ctx context.Context, cmdLine string) (context.Context, error) {
	line := strings.TrimSpace(cmdLine)
	if line == "" {
		return ctx, nil
	}
	args := reCmd.FindAllString(line, -1)
	if args != nil {
		cmdName := args[0]
		cmd, ok := gosh.commands[cmdName]
		if !ok {
			return ctx, errors.New(fmt.Sprintf("command not found: %s", cmdName))
		}
		return cmd.Exec(ctx, args)
	}
	return ctx, errors.New(fmt.Sprintf("unable to parse command line: %s", line))
}

func listGoshFiles(dir, pattern string) ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filteredFiles := []os.FileInfo{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		matched, err := regexp.MatchString(pattern, file.Name())
		if err != nil {
			return nil, err
		}
		if matched {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles, nil
}

func gosh_fxns() []string {
	keys := make([]string, 0, len(shell.commands))
	for k, _ := range shell.commands {
		keys = append(keys, k)
	}
	return keys
}

func gosh_main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "gosh.prompt", api.DefaultPrompt)
	ctx = context.WithValue(ctx, "gosh.stdout", os.Stdout)
	ctx = context.WithValue(ctx, "gosh.stderr", os.Stderr)
	ctx = context.WithValue(ctx, "gosh.stdin", os.Stdin)

	if err := shell.Init(ctx); err != nil {
		fmt.Println("\n\nfailed to initialize:\n", err)
		os.Exit(1)
	}

	// prompt for help
	cmdCount := len(shell.commands)
	if cmdCount > 0 {
		if _, ok := shell.commands["help"]; ok {
			print_ok(os.Stderr, fmt.Sprintf("\nLoaded %d command(s)...", cmdCount))
			print_ok(os.Stderr, "Type help for available commands")
		}
	} else {
		fmt.Print("\n\nNo commands found")
	}
	if false {
		go shell.Open(bufio.NewReader(os.Stdin))

		sigs := make(chan os.Signal)
		signal.Notify(sigs, syscall.SIGINT)
		select {
		case <-sigs:
			cancel()
			<-shell.Closed()
		case <-shell.Closed():
		}
	}
}
