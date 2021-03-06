package executors

import (
	"fmt"
	"github.com/abesto/easyssh/from_sexp"
	"github.com/abesto/easyssh/interfaces"
	"github.com/abesto/easyssh/target"
	"github.com/abesto/easyssh/util"
	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func Make(input string) interfaces.Executor {
	return from_sexp.MakeFromString(input, makeByName).(interfaces.Executor)
}

func SupportedExecutorNames() []string {
	var keys = make([]string, len(executorMakerMap))
	var i = 0
	for key := range executorMakerMap {
		keys[i] = key
		i += 1
	}
	return keys
}

func makeFromSExp(data []interface{}) interfaces.Executor {
	return from_sexp.Make(data, makeByName).(interfaces.Executor)
}

const (
	nameSshLogin        = "ssh-login"
	nameCsshx           = "knife"
	nameSshExec         = "ssh-exec"
	nameSshExecParallel = "ssh-exec-parallel"
	nameTmuxCssh        = "tmux-cssh"
	nameIfOneTarget     = "if-one-target"
	nameIfArgs          = "if-args"
)

var executorMakerMap = map[string]func() interfaces.Executor{
	nameSshLogin:        func() interfaces.Executor { return &sshLogin{} },
	nameCsshx:           func() interfaces.Executor { return &csshx{} },
	nameSshExec:         func() interfaces.Executor { return &sshExec{} },
	nameSshExecParallel: func() interfaces.Executor { return &sshExecParallel{} },
	nameTmuxCssh:        func() interfaces.Executor { return &tmuxCssh{} },
	nameIfOneTarget:     func() interfaces.Executor { return &oneOrMore{} },
	nameIfArgs:          func() interfaces.Executor { return &ifArgs{} },
}

func makeByName(name string) interface{} {
	var d interfaces.Executor
	for key, maker := range executorMakerMap {
		if key == name {
			d = maker()
		}
	}
	if d == nil {
		util.Abort("Executor \"%s\" is not known", name)
	}
	return d
}

func requireExactlyOneTarget(e interfaces.Executor, targets []target.Target) {
	if len(targets) != 1 {
		util.Abort("%s expects exactly one target, got %d: %s", e, len(targets), targets)
	}
}

func requireAtLeastOneTarget(e interfaces.Executor, targets []target.Target) {
	if len(targets) < 1 {
		util.Abort("%s expects at least one target.", e)
	}
}

func requireNoCommand(e interfaces.Executor, command []string) {
	if len(command) > 0 {
		util.Abort("%s doesn't accept a command, got: %s", e, command)
	}
}

func requireCommand(e interfaces.Executor, command []string) {
	if len(command) == 0 {
		util.Abort("%s requires a command.", e)
	}
}

func myExec(binaryName string, args ...string) {
	var binary = util.LookPathOrAbort(binaryName)
	var argv = append([]string{binary}, args...)
	util.Logger.Infof("Executing %s", argv)
	syscall.Exec(binary, argv, os.Environ())
}

type sshLogin struct{}

func (e *sshLogin) Exec(targets []target.Target, command []string) {
	requireExactlyOneTarget(e, targets)
	requireNoCommand(e, command)
	myExec("ssh", targets[0].String())
}
func (e *sshLogin) SetArgs(args []interface{}) {
	util.RequireNoArguments(e, args)
}
func (e *sshLogin) String() string {
	return "<ssh-login>"
}

type sshExec struct{}

func (e *sshExec) Exec(targets []target.Target, command []string) {
	requireAtLeastOneTarget(e, targets)
	requireCommand(e, command)

	for _, target := range targets {
		var binary = util.LookPathOrAbort("ssh")
		var cmd = makeLoggedCommand(binary, target, append([]string{target.String()}, command...))
		util.Logger.Infof("Executing %s", cmd.Args)
		cmd.Run()
	}
}
func (e *sshExec) SetArgs(args []interface{}) {
	util.RequireNoArguments(e, args)
}
func (e *sshExec) String() string {
	return "<ssh-exec>"
}

type sshExecParallel struct{}

func (e *sshExecParallel) Exec(targets []target.Target, command []string) {
	requireAtLeastOneTarget(e, targets)
	requireCommand(e, command)

	util.Logger.Infof("Parallelly executing %s on %s", command, targets)
	var binary = util.LookPathOrAbort("ssh")
	var cmds = []*exec.Cmd{}
	for _, target := range targets {
		var cmd = makeLoggedCommand(binary, target, append([]string{target.String()}, command...))
		cmds = append(cmds, cmd)
		util.Logger.Debugf("Executing %s", cmd.Args)
		cmd.Start()
	}

	for _, cmd := range cmds {
		var error = cmd.Wait()
		if error != nil {
			util.Logger.Errorf("%s: %s", cmd.Args, error)
		}
	}
}
func (e *sshExecParallel) SetArgs(args []interface{}) {
	util.RequireNoArguments(e, args)
}
func (e *sshExecParallel) String() string {
	return "<ssh-exec-parallel>"
}

type csshx struct{}

func (e *csshx) Exec(targets []target.Target, command []string) {
	requireAtLeastOneTarget(e, targets)
	requireNoCommand(e, command)
	myExec("csshx", target.TargetStrings(targets)...)
}
func (e *csshx) SetArgs(args []interface{}) {
	util.RequireNoArguments(e, args)
}
func (e *csshx) String() string {
	return "<csshx>"
}

type tmuxCssh struct{}

func (e *tmuxCssh) Exec(targets []target.Target, command []string) {
	requireAtLeastOneTarget(e, targets)
	requireNoCommand(e, command)
	myExec("tmux-cssh", target.TargetStrings(targets)...)
}
func (e *tmuxCssh) SetArgs(args []interface{}) {
	util.RequireNoArguments(e, args)
}
func (e *tmuxCssh) String() string {
	return "<tmux-cssh>"
}

type oneOrMore struct {
	one  interfaces.Executor
	more interfaces.Executor
}

func (e *oneOrMore) Exec(targets []target.Target, command []string) {
	requireAtLeastOneTarget(e, targets)
	if len(targets) == 1 {
		util.Logger.Debugf("%s got one target, using %s", e, e.one)
		e.one.Exec(targets, command)
	} else {
		util.Logger.Debugf("%s got more than one target, using %s", e, e.more)
		e.more.Exec(targets, command)
	}
}
func (e *oneOrMore) SetArgs(args []interface{}) {
	util.RequireArguments(e, 2, args)
	e.one = makeFromSExp(args[0].([]interface{}))
	e.more = makeFromSExp(args[1].([]interface{}))
}
func (e *oneOrMore) String() string {
	return fmt.Sprintf("<one-or-more %s %s>", e.one, e.more)
}

type ifArgs struct {
	withArgs    interfaces.Executor
	withoutArgs interfaces.Executor
}

func (e *ifArgs) Exec(targets []target.Target, args []string) {
	if len(args) < 1 {
		util.Logger.Debugf("%s got no args, using %s", e, e.withoutArgs)
		e.withoutArgs.Exec(targets, args)
	} else {
		util.Logger.Debugf("%s got args, using %s", e, e.withArgs)
		e.withArgs.Exec(targets, args)
	}
}
func (e *ifArgs) SetArgs(args []interface{}) {
	util.RequireArguments(e, 2, args)
	e.withArgs = makeFromSExp(args[0].([]interface{}))
	e.withoutArgs = makeFromSExp(args[1].([]interface{}))
}
func (e *ifArgs) String() string {
	return fmt.Sprintf("<if-args %s %s>", e.withArgs, e.withoutArgs)
}

type prefixedLogWriterProxy struct {
	prefix string
	logger *golog.Logger
}

func newPrefixedLogWriterProxy(prefix string, file *os.File) prefixedLogWriterProxy {
	return prefixedLogWriterProxy{prefix: prefix, logger: golog.New(file, log.Debug)}
}
func (w prefixedLogWriterProxy) Write(p []byte) (n int, err error) {
	var logger = *w.logger
	var lines = strings.Split(strings.TrimSpace(string(p)), "\n")
	for _, line := range lines {
		logger.Notice(w.prefix, line)
	}
	return len(p), nil
}

func makeLoggedCommand(binary string, target target.Target, args []string) *exec.Cmd {
	var cmd = exec.Command(binary, args...)
	var prefixStdout = fmt.Sprintf("[%s] (STDOUT)", target)
	var prefixStderr = fmt.Sprintf("[%s] (STDERR)", target)

	cmd.Stdout = newPrefixedLogWriterProxy(prefixStdout, os.Stdout)
	cmd.Stderr = newPrefixedLogWriterProxy(prefixStderr, os.Stderr)
	cmd.Env = os.Environ()

	return cmd
}
