package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

// Function constructor - constructs new function for listing given directory
func listFiles(path string) func(string) []string {
	return func(line string) []string {
		names := make([]string, 0)
		files, _ := ioutil.ReadDir(path)
		for _, f := range files {
			names = append(names, f.Name())
		}
		return names
	}
}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("gosh",
		readline.PcItem("list", readline.PcItem("-f"), readline.PcItem("-p")),
		readline.PcItem("build",
			readline.PcItem("all"),
			readline.PcItem("file"),
		),
		readline.PcItem("load"),
		readline.PcItem("exec"),
		readline.PcItem("build"),
	),
	readline.PcItem("mode",
		readline.PcItem("vi"),
		readline.PcItem("emacs"),
	),
	readline.PcItem("login"),
	readline.PcItem("say",
		readline.PcItemDynamic(listFiles("./"),
			readline.PcItem("with",
				readline.PcItem("following"),
				readline.PcItem("items"),
			),
		),
		readline.PcItem("hello"),
		readline.PcItem("bye"),
	),
	readline.PcItem("setprompt"),
	readline.PcItem("setpassword"),
	readline.PcItem("bye"),
	readline.PcItem("help"),
	readline.PcItem("go",
		readline.PcItem("build", readline.PcItem("-o"), readline.PcItem("-v")),
		readline.PcItem("install",
			readline.PcItem("-v"),
			readline.PcItem("-vv"),
			readline.PcItem("-vvv"),
		),
		readline.PcItem("test"),
	),
	readline.PcItem("sleep"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

func get_prompt() string {
	return "\033[31m»\033[0m "
}
func gosh_mode(line string) {
	fmt.Fprintf(os.Stderr, "GOSH MODE %s>\n", line)
}

func main() {
	gosh_main()
	l, err := readline.NewEx(&readline.Config{
		Prompt:          get_prompt(),
		HistoryFile:     "/tmp/readline.tmp",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})

	log.SetOutput(l.Stderr())
	log.Println("log initialized...")
	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "mode "):
			switch line[5:] {
			case "vi":
				l.SetVimMode(true)
			case "emacs":
				l.SetVimMode(false)
			default:
				println("invalid mode:", line[5:])
			}
		case strings.HasPrefix(line, "gosh "):
			msg := `unknown`
			title := `unknown`
			switch line[5:] {
			case "build all":
				build_cmds := shell.buildFileCommands()
				title = fmt.Sprintf(`%d GOSH %s`, len(build_cmds), `Commands`)
				dur, err := shell.buildFiles()
				if err != nil {
					panic(err)
				}
				msg = fmt.Sprintf("Built in %s", dur)
			case "build commands":
				build_cmds := shell.buildFileCommands()
				title = fmt.Sprintf(`%d GOSH %s`, len(build_cmds), `Commands`)
				msg = fmt.Sprintf("%s", strings.Join(build_cmds, "\n"))
			case "load":
				shell.loadCommands()
				fxns := gosh_fxns()
				title = fmt.Sprintf("%d Functions- %s\n", len(fxns), fxns)
				msg = fmt.Sprintf("%d Functions- %s\n", len(fxns), fxns)
			case "list -f":
				gosh_files := shell.listFiles()
				title = fmt.Sprintf(`%d GOSH %s`, len(gosh_files), `Files`)
				msg = fmt.Sprintf("%s", strings.Join(gosh_files, `, `))
			case "list -p":
				plugins := shell.listPlugins()
				title = fmt.Sprintf(`%d GOSH %s`, len(plugins), `Plugins`)
				msg = fmt.Sprintf("%s", strings.Join(plugins, `, `))
			default:
				msg = fmt.Sprintf("invalid mode:", line[5:])
				panic(msg)
			}
			print_ok_title(os.Stderr, title)
			print_ok(os.Stderr, msg)
		case line == "mode":
			if l.IsVimMode() {
				println("current mode: vim")
			} else {
				println("current mode: emacs")
			}
		case line == "login":
			pswd, err := l.ReadPassword("please enter your password: ")
			if err != nil {
				break
			}
			println("you enter:", strconv.Quote(string(pswd)))
		case line == "help":
			usage(l.Stderr())
		case line == "setpassword":
			pswd, err := l.ReadPasswordWithConfig(setPasswordCfg)
			if err == nil {
				println("you set:", strconv.Quote(string(pswd)))
			}
		case strings.HasPrefix(line, "setprompt"):
			if len(line) <= 10 {
				log.Println("setprompt <prompt>")
				break
			}
			l.SetPrompt(line[10:])
		case strings.HasPrefix(line, "say"):
			line := strings.TrimSpace(line[3:])
			if len(line) == 0 {
				log.Println("say what?")
				break
			}
			go func() {
				for range time.Tick(time.Second) {
					log.Println(line)
				}
			}()
		case line == "bye":
			goto exit
		case line == "sleep":
			log.Println("sleep 4 second")
			time.Sleep(4 * time.Second)
		case line == "":
		default:
			log.Println("you said:", strconv.Quote(line))
		}
	}
exit:
}
