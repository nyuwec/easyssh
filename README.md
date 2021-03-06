# easyssh

`easyssh` is the culmination of several years of SSH-related aliases. It's a highly configurable wrapper around `ssh`, `tmux-cssh`, `csshx`, `aws`, `knife` and who knows what else. It's for you if the following makes you excited. You can have a single alias that does this (let's call the alias `s`):

 * `s myhost.com` logs in to myhost.com
 * `s app.myhost.com,db.myhost.com` logs in to both hosts using `tmux-ssh`
 * `s -lroot myhost.com /etc/init.d/apache2 reload` reloads apache
 * `s app.myhost.com,db.myhost.com uptime` runs uptime on both hosts (parallelly, which is interesting if you run longer-running commands)
 * `s -lroot roles:app /etc/init.d/apache2 reload` parallelly reloads apache on all nodes that have the role `app` in Chef
 * If you provide a hostname that looks like it includes an EC2 instance id, it uses the `aws` CLI tool to look up the public IP, and uses that.

The syntax is slightly verbose, it's designed to be used in aliases you frequently need.

## Installation

```sh
go get github.com/abesto/easyssh
```

### If you're new to Go

You can follow the official [Getting Started](http://golang.org/doc/install) guide.

The short version, for OSX:

```sh
brew install go
mkdir ~/.gocode
echo "export GOPATH=$HOME/.gocode" >> ~/.bashrc
```

After installation you will find the executable in `~/.gocode/bin`.

## Simple usage

You probably won't ever do this; it's just a basic demonstration of the syntax.

```sh
# log in with an interactive shell; old-fashioned ssh
easyssh myhost.com
# sequentially run "hostname" on all nodes matching "roles:app" according to knife
easyssh -c='(ssh-exec)' -d='(knife)' roles:app hostname
# parallelly reload apache on all your app servers (as root, of course)
easyssh -c='(ssh-exec-parallel)' -d='(knife)' -l=root roles:app /etc/init.d/apache2 reload
```

## Example alias

This one alias implements the use-case described in the introduction.

```sh
easyssh_executor='(if-args (ssh-exec-parallel) (if-one-target (ssh-login) (tmux-cssh)))'
easyssh_discoverer='(first-matching (knife) (comma-separated))'
easyssh_filter='(list (ec2-instance-id us-east-1) (ec2-instance-id us-west-1))'
alias s="easyssh -e='$easyssh_cmd' -d='$easyssh_discoverer' -f='$easyssh_filter'"
```

If you frequently log in to servers as root, you can then go:

```sh
alias sr='s -l root'
# reload apache on app servers (as root)
sr roles:app /etc/init.d/apache2 reload
```

This assumes that

 * `knife` is correctly configured for the Chef environment you want to work with
 * The `aws` CLI tool is correctly configured

## Configuration

The behavior of `easyssh` is controlled by three components:

 * *Discoverers* produce a list of targets (user, host pairs) from some input string; the input string is the first
   non-flag argument to `easyssh`
 * *Filters* mutate the list of targets produced by the discoverers
 * *Executors* do something with the targets, optionally taking the arguments not consumed by the discoverers.

An `easyssh` command then looks like this:

```
easyssh -e='DISCOVERER_DEFINITION' -f='FILTER_DEFINITION' -e='EXECUTOR_DEFINITION' DISCOVERER_ARG [EXECUTOR [ARGUMENT [...]]]
```

Each definition is an S-Expression; the terms usable in the S-Expressions are described below.

### Discoverers

| Name      | Arguments   | Description |
|-----------|-------------|-------------|
| `comma-separated` | - | Takes the discoverer argument, splits it at commas, and uses the resulting strings as the target hostnames. |
| `knife` | - | Passes the discoverer argument to `knife search node`, and returns the public IP addresses provided by Chef as target hostnames. |
| `first-matching` | Any number of discoverers | Runs the discoverers in its argument list in the order they were provided, and uses the first resulting non-empty target list. |

### Filters

| Name      | Arguments   | Description |
|-----------|-------------|-------------|
| `id` | - | Doesn't touch the the target list. |
| `first` | - | Drops all targets in the target list, except for the first one. |
| `ec2-instance-id` | AWS region | For each target in the target list, it looks for an EC2 instance id in the target name. If there is one, it uses `aws` to look up its public IP and replace the target name with that. |
| `list` | Any number of filters | Applies each filter in its arguments to the target list. |

### Executors

| Name      | Arguments   | Required targets | Command | Description |
|-----------|-------------|------------------|---------|-------------|
| `ssh-login` | - | 1 | rejects | Logs in to the target using SSH |
| `ssh-exec` | - | >0 | requires | Executes the command on each target sequentially |
| `ssh-exec-parallel` | - | >0 | requires | Executes the command on each target parallelly |
| `csshx` | - | >0 | rejects | Uses `csshx` to log in to all the targets |
| `tmux-cssh` | - | >0 | rejects | Uses `tmux-cssh` to log in to all the targets |
| `if-one-target` | exactly two executors | >0 | pass-through | If there's one target, it calls the executor in its first argument. Otherwise the executor in its second argument. |
| `if-args` | exactly two executors | N/A | pass-through | If a command was defined, it calls the executor in its first argument. Otherwise the executor in its second argument. |
