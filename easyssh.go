package main

import (
	"flag"
	"github.com/abesto/easyssh/discoverers"
	"github.com/abesto/easyssh/executors"
	"github.com/abesto/easyssh/filters"
	"github.com/abesto/easyssh/interfaces"
	"github.com/abesto/easyssh/target"
	"github.com/abesto/easyssh/util"
	"github.com/alexcesaro/log/stdlog"
	"fmt"
	"os"
	"strings"
)

func main() {
	var (
		discovererDefinition string
		discoverer           interfaces.Discoverer
		executorDefinition   string
		executor             interfaces.Executor
		user                 string
		filterDefinition     string
		filter               interfaces.TargetFilter
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [options] target-definition [command]

Where
  target-definition is the input to the discoverer(s) defined with -d
  command, if provided, will be run on the targets

Ideally single alias should cover all your use-cases. For example:
  smartssh_executor='(if-args (ssh-exec-parallel) (if-one-target (ssh-login) (tmux-cssh)))'
  smartssh_discoverer='(first-matching (knife) (comma-separated))'
  smartssh_filter='(list (ec2-instance-id us-east-1) (ec2-instance-id us-west-1))'
  alias s="%s -e='$smartssh_cmd' -d='$smartssh_discoverer' -f='$smartssh_filter'"

Configuration details:
  open https://github.com/abesto/smartssh/blob/master/README.md#configuration

Options:
`, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&user, "l", "",
		"Specifies the user to log in as on the remote machine.")
	flag.StringVar(&discovererDefinition, "d", "(comma-separated)",
		fmt.Sprintf("Discoverer definition. Supported discoverers: %s", strings.Join(discoverers.SupportedDiscovererNames(), ", ")))
	flag.StringVar(&executorDefinition, "e", "(ssh-login)",
		fmt.Sprintf("Executor definition. Supported executors: %s", strings.Join(executors.SupportedExecutorNames(), ", ")))
	flag.StringVar(&filterDefinition, "f", "(id)",
		fmt.Sprintf("Filter definition. Supported filters: %s", strings.Join(filters.SupportedFilterNames(), ", ")))
	flag.Parse()

	util.Logger = stdlog.GetFromFlags()
	var logger = util.Logger

	if flag.NArg() == 0 {
		util.Abort("Required argument for target host lookup missing")
	}

	discoverer = discoverers.Make(discovererDefinition)
	executor = executors.Make(executorDefinition)
	filter = filters.Make(filterDefinition)

	var targets []target.Target = []target.Target{}
	for _, host := range discoverer.Discover(flag.Arg(0)) {
		targets = append(targets, target.Target{Host: host, User: user})
	}
	if len(targets) == 0 {
		util.Abort("No targets found")
	}

	logger.Debugf("Targets before filters: %s", targets)
	targets = filter.Filter(targets)
	logger.Infof("Targets: %s", targets)

	var command = flag.Args()[1:]
	executor.Exec(targets, command)
}
