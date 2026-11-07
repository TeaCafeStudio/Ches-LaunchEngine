package terminal

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/TeaCafeStudio/Ches-LaunchEngine/internal/config"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/core"
	"github.com/TeaCafeStudio/Ches-LaunchEngine/pkg/models"
	"golang.org/x/term"
)

var ErrExit = errors.New("terminal exit")

type Options struct {
	Engine              core.DebugEngine
	Accounts            core.AccountManager
	Config              config.Config
	Version             string
	HistoryLimit        int
	DefaultLaunchConfig string
	Input               io.Reader
	InputFile           *os.File
	Output              io.Writer
}

type Handler func(*Terminal, []string) error

type Registry struct {
	commands map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Handler)}
}

func (registry *Registry) Register(name string, handler Handler) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " \t\r\n") {
		return errors.New("command name must be a non-empty word")
	}
	if handler == nil {
		return errors.New("command handler is required")
	}
	if _, exists := registry.commands[name]; exists {
		return fmt.Errorf("command %q is already registered", name)
	}
	registry.commands[name] = handler
	return nil
}

func (registry *Registry) Names() []string {
	names := make([]string, 0, len(registry.commands))
	for name := range registry.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type Terminal struct {
	engine              core.DebugEngine
	accounts            core.AccountManager
	cfg                 config.Config
	version             string
	historyLimit        int
	defaultLaunchConfig string
	input               io.Reader
	inputFile           *os.File
	output              io.Writer
	registry            *Registry

	mu             sync.Mutex
	ctx            context.Context
	line           []rune
	pending        []byte
	interactive    bool
	history        []string
	historyCursor  int
	watchFilter    core.EventFilter
	watchEnabled   bool
	displayEvents  bool
	nonInteractive bool
	currentTaskID  string
}

func New(options Options) (*Terminal, error) {
	if options.Engine == nil {
		return nil, errors.New("debug terminal requires a Core debug engine")
	}
	if options.Config.Version == 0 {
		options.Config = config.Default()
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}
	if options.Input == nil {
		options.Input = os.Stdin
	}
	if options.HistoryLimit < 0 {
		return nil, errors.New("history limit cannot be negative")
	}
	if options.HistoryLimit == 0 {
		options.HistoryLimit = options.Config.DebugTerminal.HistoryLimit
	}
	if options.Version == "" {
		options.Version = "dev"
	}
	terminal := &Terminal{
		engine: options.Engine, accounts: options.Accounts, cfg: options.Config, version: options.Version,
		historyLimit: options.HistoryLimit, defaultLaunchConfig: options.DefaultLaunchConfig,
		input:     options.Input,
		inputFile: options.InputFile, output: options.Output,
		registry: NewRegistry(), historyCursor: -1, displayEvents: true,
	}
	if terminal.inputFile == nil {
		terminal.inputFile, _ = terminal.input.(*os.File)
	}
	if err := terminal.registerCommands(); err != nil {
		return nil, err
	}
	return terminal, nil
}

func (terminal *Terminal) Registry() *Registry {
	return terminal.registry
}

func (terminal *Terminal) Execute(line string) (bool, error) {
	words, err := Parse(line)
	if err != nil {
		return true, err
	}
	if len(words) == 0 {
		return true, nil
	}
	handler, exists := terminal.registry.commands[words[0]]
	if !exists {
		return true, fmt.Errorf("unknown command %q; run help", words[0])
	}
	err = handler(terminal, words[1:])
	if errors.Is(err, ErrExit) {
		return false, nil
	}
	return true, err
}

func (terminal *Terminal) Run(ctx context.Context) error {
	events, unsubscribe, err := terminal.engine.SubscribeEvents(512)
	if err != nil {
		return fmt.Errorf("subscribe to engine events: %w", err)
	}
	defer unsubscribe()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	terminal.mu.Lock()
	terminal.ctx = ctx
	terminal.mu.Unlock()
	defer func() {
		terminal.mu.Lock()
		terminal.ctx = nil
		terminal.mu.Unlock()
	}()
	eventDone := make(chan struct{})
	go func() {
		defer close(eventDone)
		for {
			select {
			case <-ctx.Done():
				return
			case event, open := <-events:
				if !open {
					return
				}
				if terminal.acceptEvent(event) {
					terminal.printEvent(event)
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-eventDone
	}()

	outputFile, hasOutputFile := terminal.output.(*os.File)
	if terminal.inputFile != nil && hasOutputFile &&
		term.IsTerminal(int(terminal.inputFile.Fd())) && term.IsTerminal(int(outputFile.Fd())) {
		return terminal.runInteractive(ctx)
	}
	return terminal.runNonInteractive(ctx)
}

func (terminal *Terminal) runNonInteractive(ctx context.Context) error {
	terminal.mu.Lock()
	terminal.nonInteractive = true
	terminal.mu.Unlock()
	defer func() {
		terminal.mu.Lock()
		terminal.nonInteractive = false
		terminal.mu.Unlock()
	}()

	type inputLine struct {
		line string
		err  error
		done bool
	}
	lines := make(chan inputLine, 1)
	go func() {
		scanner := bufio.NewScanner(terminal.input)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			select {
			case lines <- inputLine{line: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		select {
		case lines <- inputLine{err: scanner.Err(), done: true}:
		case <-ctx.Done():
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case input := <-lines:
			if input.done {
				if input.err != nil {
					return fmt.Errorf("read debug command: %w", input.err)
				}
				return nil
			}
			terminal.remember(input.line)
			keepGoing, err := terminal.Execute(input.line)
			if err != nil {
				terminal.printLine("error: " + terminal.printErrorDetails(err))
			}
			if !keepGoing {
				return nil
			}
		}
	}
}

func (terminal *Terminal) runInteractive(ctx context.Context) error {
	state, err := term.MakeRaw(int(terminal.inputFile.Fd()))
	if err != nil {
		return fmt.Errorf("enable interactive terminal input: %w", err)
	}
	defer func() {
		if restoreErr := term.Restore(int(terminal.inputFile.Fd()), state); restoreErr != nil {
			terminal.printLine("warning: restore terminal mode: " + restoreErr.Error())
		}
	}()
	terminal.mu.Lock()
	terminal.interactive = true
	terminal.mu.Unlock()
	defer func() {
		terminal.mu.Lock()
		terminal.interactive = false
		terminal.mu.Unlock()
	}()
	terminal.printLine("ChesLaunchEngine Debug Terminal. Type help for commands.")
	terminal.drawPrompt()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var buffer [1]byte
		if _, err := terminal.inputFile.Read(buffer[:]); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read terminal input: %w", err)
		}
		switch buffer[0] {
		case '\r', '\n':
			line := terminal.currentLine()
			terminal.mu.Lock()
			terminal.line = nil
			terminal.pending = nil
			terminal.historyCursor = -1
			terminal.mu.Unlock()
			terminal.outputLocked("\r\n")
			terminal.remember(line)
			keepGoing, commandErr := terminal.Execute(line)
			if commandErr != nil {
				terminal.printLine("error: " + terminal.printErrorDetails(commandErr))
			}
			if !keepGoing {
				return nil
			}
			terminal.drawPrompt()
		case 3:
			if err := terminal.handleInterrupt(ctx); err != nil {
				if errors.Is(err, ErrExit) {
					return nil
				}
				terminal.printLine("error: " + err.Error())
			}
		case 4:
			if terminal.currentLine() == "" {
				return nil
			}
		case 8, 127:
			terminal.mu.Lock()
			if len(terminal.pending) > 0 {
				terminal.pending = nil
			} else if len(terminal.line) > 0 {
				terminal.line = terminal.line[:len(terminal.line)-1]
			}
			terminal.mu.Unlock()
			terminal.redrawPrompt()
		case 27:
			if err := terminal.handleEscape(); err != nil {
				return err
			}
		default:
			if buffer[0] >= 32 {
				terminal.appendInputByte(buffer[0])
			}
		}
	}
}

func (terminal *Terminal) appendInputByte(value byte) {
	terminal.mu.Lock()
	terminal.pending = append(terminal.pending, value)
	var output string
	if utf8.FullRune(terminal.pending) {
		character, size := utf8.DecodeRune(terminal.pending)
		terminal.line = append(terminal.line, character)
		terminal.pending = terminal.pending[size:]
		output = string(character)
	}
	terminal.mu.Unlock()
	if output != "" {
		terminal.outputLocked(output)
	}
}

func (terminal *Terminal) handleEscape() error {
	var sequence [2]byte
	if _, err := io.ReadFull(terminal.inputFile, sequence[:]); err != nil {
		return nil
	}
	if sequence[0] != '[' {
		return nil
	}
	terminal.mu.Lock()
	switch sequence[1] {
	case 'A':
		if len(terminal.history) > 0 {
			if terminal.historyCursor < 0 {
				terminal.historyCursor = len(terminal.history) - 1
			} else if terminal.historyCursor > 0 {
				terminal.historyCursor--
			}
			terminal.line = []rune(terminal.history[terminal.historyCursor])
			terminal.pending = nil
		}
	case 'B':
		if terminal.historyCursor >= 0 && terminal.historyCursor < len(terminal.history)-1 {
			terminal.historyCursor++
			terminal.line = []rune(terminal.history[terminal.historyCursor])
			terminal.pending = nil
		} else {
			terminal.historyCursor = -1
			terminal.line = nil
			terminal.pending = nil
		}
	}
	terminal.mu.Unlock()
	terminal.redrawPrompt()
	return nil
}

func (terminal *Terminal) handleInterrupt(ctx context.Context) error {
	line := terminal.currentLine()
	if line != "" {
		terminal.mu.Lock()
		terminal.line = nil
		terminal.pending = nil
		terminal.mu.Unlock()
		terminal.outputLocked("^C\r\n")
		terminal.drawPrompt()
		return nil
	}
	terminal.mu.Lock()
	taskID := terminal.currentTaskID
	terminal.mu.Unlock()
	if taskID != "" {
		task, err := terminal.engine.GetLaunchStatus(taskID)
		if err == nil {
			switch task.Status {
			case models.LaunchResolving, models.LaunchValidating, models.LaunchDownloading, models.LaunchPreparing, models.LaunchLaunching:
				if err := terminal.engine.CancelTask(ctx, taskID); err != nil {
					return err
				}
				terminal.printLine("^C cancelled launch task " + taskID)
				terminal.mu.Lock()
				terminal.currentTaskID = ""
				terminal.mu.Unlock()
				terminal.drawPrompt()
				return nil
			case models.LaunchRunning:
				if err := terminal.engine.StopProcess(ctx, task.ProcessID, false); err != nil {
					return err
				}
				terminal.printLine("^C requested graceful stop for process " + fmt.Sprint(task.ProcessID))
				terminal.mu.Lock()
				terminal.currentTaskID = ""
				terminal.mu.Unlock()
				terminal.drawPrompt()
				return nil
			}
		}
	}
	terminal.outputLocked("^C\r\n")
	return ErrExit
}

func (terminal *Terminal) currentLine() string {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	return string(terminal.line)
}

func (terminal *Terminal) remember(line string) {
	line = strings.TrimSpace(line)
	if line == "" || terminal.historyLimit == 0 {
		return
	}
	terminal.mu.Lock()
	terminal.history = append(terminal.history, line)
	if len(terminal.history) > terminal.historyLimit {
		terminal.history = append([]string(nil), terminal.history[len(terminal.history)-terminal.historyLimit:]...)
	}
	terminal.historyCursor = -1
	terminal.mu.Unlock()
}

func (terminal *Terminal) printLine(value string) {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if terminal.interactive {
		fmt.Fprint(terminal.output, "\r\x1b[2K")
	}
	fmt.Fprintln(terminal.output, value)
	if terminal.interactive {
		fmt.Fprintf(terminal.output, "cle> %s", string(terminal.line))
	}
}

func (terminal *Terminal) outputLocked(value string) {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	fmt.Fprint(terminal.output, value)
}

func (terminal *Terminal) drawPrompt() {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	fmt.Fprintf(terminal.output, "cle> %s", string(terminal.line))
}

func (terminal *Terminal) redrawPrompt() {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	fmt.Fprintf(terminal.output, "\r\x1b[2Kcle> %s", string(terminal.line))
}

func (terminal *Terminal) acceptEvent(event core.Event) bool {
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if !terminal.displayEvents {
		return false
	}
	if !terminal.watchEnabled {
		return true
	}
	if terminal.watchFilter.TaskID != "" && terminal.watchFilter.TaskID != event.TaskID {
		return false
	}
	if terminal.watchFilter.ProcessID != 0 && terminal.watchFilter.ProcessID != event.ProcessID {
		return false
	}
	return true
}

func (terminal *Terminal) printEvent(event core.Event) {
	parts := []string{string(event.Source)}
	if event.TaskID != "" {
		parts = append(parts, "task="+event.TaskID)
	}
	if event.ProcessID != 0 {
		parts = append(parts, "pid="+fmt.Sprint(event.ProcessID))
	}
	if event.Level != "" {
		parts = append(parts, event.Level)
	}
	message := event.Message
	if code := event.Data["errorCode"]; code != "" {
		message = code + ": " + message
	}
	if event.Source == core.EventDownload {
		received := event.Data["receivedBytes"]
		total := event.Data["totalBytes"]
		progress := ""
		if total != "" && total != "0" {
			progress = " " + received + "/" + total + " bytes"
		} else if received != "" {
			progress = " " + received + " bytes"
		}
		done, totalFiles := event.Data["filesDone"], event.Data["filesTotal"]
		if done != "" && totalFiles != "" {
			progress += fmt.Sprintf(" (%s/%s files)", done, totalFiles)
		}
		message += progress
	}
	terminal.printLine("[" + strings.Join(parts, " ") + "] " + message)
}
