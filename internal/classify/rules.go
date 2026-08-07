package classify

import "strings"

// The rule tables. Four kinds, checked in this order by classifyWords:
//
//	alwaysDestructive — the command mutates regardless of arguments.
//	flagGated         — the verdict depends on flags (sed -i, find -delete).
//	subcommandTables  — the verdict depends on a subcommand (git, docker).
//	safeCommands      — read-only or purely additive.
//
// Anything in none of them is Unknown, which confirms. Adding a command to
// safeCommands is therefore the only change that can create a false
// negative, so that table is the one to be careful with.

// shellKeywords are skipped when they appear in command position; the real
// command follows them (`if rm …`, `do rm …`, `! rm …`).
var shellKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true,
	"while": true, "until": true, "do": true, "done": true,
	"for": true, "case": true, "esac": true, "select": true,
	"function": true, "!": true, "{": true, "}": true,
}

var alwaysDestructive = map[string]string{
	// Deleting and overwriting files.
	"rm":       "rm deletes files",
	"rmdir":    "rmdir removes directories",
	"unlink":   "unlink deletes a file",
	"shred":    "shred overwrites file contents irrecoverably",
	"srm":      "srm securely deletes files",
	"truncate": "truncate resizes files in place",
	"mv":       "mv moves or renames files, overwriting the destination",
	"cp":       "cp overwrites files at the destination",
	"rsync":    "rsync writes to the destination and can delete there",
	"scp":      "scp writes files to the destination",
	"sftp":     "sftp transfers and can delete remote files",
	"install":  "install copies files into place",
	"ln":       "ln creates or replaces links",
	"rename":   "rename renames files in bulk",

	// Whole-device and filesystem operations.
	"dd":       "dd writes raw blocks to a device or file",
	"fdisk":    "fdisk rewrites the partition table",
	"sfdisk":   "sfdisk rewrites the partition table",
	"gdisk":    "gdisk rewrites the partition table",
	"sgdisk":   "sgdisk rewrites the partition table",
	"parted":   "parted rewrites the partition table",
	"diskutil": "diskutil repartitions or erases disks",
	"mkswap":   "mkswap reformats a swap device",
	"swapon":   "swapon changes active swap devices",
	"swapoff":  "swapoff changes active swap devices",
	"mount":    "mount changes what is mounted",
	"umount":   "umount unmounts a filesystem",
	"fsck":     "fsck repairs a filesystem in place",

	// Permissions and ownership.
	"chmod":   "chmod changes file permissions",
	"chown":   "chown changes file ownership",
	"chgrp":   "chgrp changes file group ownership",
	"chflags": "chflags changes file flags",
	"chattr":  "chattr changes file attributes",
	"setfacl": "setfacl changes access control lists",
	"xattr":   "xattr changes extended attributes",

	// Processes and machine state.
	"kill":           "kill signals a running process",
	"killall":        "killall signals processes by name",
	"pkill":          "pkill signals processes by pattern",
	"xkill":          "xkill terminates a window's process",
	"reboot":         "reboot restarts the machine",
	"shutdown":       "shutdown powers the machine down",
	"halt":           "halt stops the machine",
	"poweroff":       "poweroff powers the machine down",
	"chroot":         "chroot runs a command in another root",
	"softwareupdate": "softwareupdate installs system updates",

	// Accounts and privileges.
	"passwd":   "passwd changes a password",
	"useradd":  "useradd creates a user account",
	"userdel":  "userdel deletes a user account",
	"usermod":  "usermod modifies a user account",
	"adduser":  "adduser creates a user account",
	"deluser":  "deluser deletes a user account",
	"groupadd": "groupadd creates a group",
	"groupdel": "groupdel deletes a group",
	"chsh":     "chsh changes a login shell",
	"visudo":   "visudo edits the sudoers file",

	// Network and firewall configuration.
	"iptables":     "iptables changes firewall rules",
	"ip6tables":    "ip6tables changes firewall rules",
	"nft":          "nft changes firewall rules",
	"ufw":          "ufw changes firewall rules",
	"pfctl":        "pfctl changes firewall rules",
	"route":        "route changes the routing table",
	"networksetup": "networksetup changes network configuration",
}

// flagGated commands whose verdict depends on their arguments.
var flagGated = map[string]func(args []string) Result{
	"sed":  sedRule,
	"perl": perlRule,
	"find": findRule,
	"curl": curlRule,
	"wget": wgetRule,
	"tee":  teeRule,
	"tar":  tarRule,
	"unzip": func(args []string) Result {
		if hasAnyFlag(args, "-l", "-t", "-v", "-z") {
			return Result{Risk: Safe}
		}
		return Result{Destructive, "unzip writes extracted files, overwriting existing ones"}
	},
	"zip": func(args []string) Result {
		return Result{Destructive, "zip writes an archive file"}
	},
	"gzip":  compressRule("gzip"),
	"bzip2": compressRule("bzip2"),
	"xz":    compressRule("xz"),
	"zstd":  compressRule("zstd"),
	"gunzip": func(args []string) Result {
		if hasAnyFlag(args, "-c", "--stdout", "-l", "--list", "-t", "--test") {
			return Result{Risk: Safe}
		}
		return Result{Destructive, "gunzip replaces the compressed file with its contents"}
	},
	"crontab": func(args []string) Result {
		if hasAnyFlag(args, "-l", "--list") {
			return Result{Risk: Safe}
		}
		return Result{Destructive, "crontab replaces or removes your scheduled jobs"}
	},
	"history": func(args []string) Result {
		if hasAnyFlag(args, "-c", "-d", "-w", "-r", "-p", "-s") {
			return Result{Destructive, "history rewrites your shell history"}
		}
		return Result{Risk: Safe}
	},
	"jq": inPlaceRule("jq"),
	"yq": inPlaceRule("yq"),
	"pacman": func(args []string) Result {
		for _, a := range args {
			if strings.HasPrefix(a, "-") && strings.ContainsAny(a, "SRUY") {
				return Result{Destructive, "pacman installs or removes packages"}
			}
		}
		return Result{Risk: Safe}
	},
	"ifconfig": func(args []string) Result {
		if len(args) > 1 {
			return Result{Destructive, "ifconfig changes network interface configuration"}
		}
		return Result{Risk: Safe}
	},
	"ssh": func(args []string) Result {
		return Result{Unknown, "ssh runs commands on another host"}
	},
	"make": func(args []string) Result {
		return Result{Unknown, "make runs whatever the Makefile defines"}
	},
	"python":  scriptRunner("python"),
	"python3": scriptRunner("python3"),
	"node":    scriptRunner("node"),
	"ruby":    scriptRunner("ruby"),
	"php":     scriptRunner("php"),
	"osascript": func(args []string) Result {
		return Result{Unknown, "osascript runs an arbitrary script"}
	},
	"source": func(args []string) Result {
		return Result{Unknown, "source runs every command in a file"}
	},
	".": func(args []string) Result {
		return Result{Unknown, "sourcing a file runs every command in it"}
	},
}

func sedRule(args []string) Result {
	for _, a := range args {
		if a == "--in-place" || strings.HasPrefix(a, "--in-place=") {
			return Result{Destructive, "sed -i edits files in place"}
		}
		// -i may be bundled (-ni) or carry a backup suffix (-i.bak); a long
		// option is never a bundle.
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 'i') {
			return Result{Destructive, "sed -i edits files in place"}
		}
	}
	return Result{Risk: Safe}
}

func perlRule(args []string) Result {
	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 'i') {
			return Result{Destructive, "perl -i edits files in place"}
		}
	}
	return Result{Unknown, "perl runs an arbitrary script"}
}

// findDestructiveActions are find primaries that do something other than
// print. -exec/-ok run an arbitrary command; the rest write or delete.
var findDestructiveActions = map[string]string{
	"-delete":  "find -delete removes every match",
	"-exec":    "find -exec runs a command on every match",
	"-execdir": "find -execdir runs a command on every match",
	"-ok":      "find -ok runs a command on every match",
	"-okdir":   "find -okdir runs a command on every match",
	"-fls":     "find -fls writes results to a file",
	"-fprint":  "find -fprint writes results to a file",
	"-fprint0": "find -fprint0 writes results to a file",
	"-fprintf": "find -fprintf writes results to a file",
}

func findRule(args []string) Result {
	for _, a := range args {
		if reason, ok := findDestructiveActions[a]; ok {
			return Result{Destructive, reason}
		}
	}
	return Result{Risk: Safe}
}

func curlRule(args []string) Result {
	for i, a := range args {
		switch {
		case a == "-o" || a == "--output" || a == "-O" || a == "--remote-name" ||
			a == "-J" || a == "--remote-header-name" || a == "--create-dirs" ||
			strings.HasPrefix(a, "--output="):
			return Result{Destructive, "curl writes the response to a file"}
		case a == "-T" || a == "--upload-file":
			return Result{Destructive, "curl uploads a file to the remote server"}
		case a == "-X" || a == "--request":
			if i+1 < len(args) && !isReadOnlyHTTPMethod(args[i+1]) {
				return Result{Destructive, "curl sends a " + args[i+1] + " request that can change remote state"}
			}
		case a == "-d" || a == "--data" || strings.HasPrefix(a, "--data-") ||
			a == "-F" || a == "--form":
			return Result{Destructive, "curl posts data to the remote server"}
		}
	}
	return Result{Risk: Safe}
}

func isReadOnlyHTTPMethod(m string) bool {
	switch strings.ToUpper(m) {
	case "GET", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func wgetRule(args []string) Result {
	if hasAnyFlag(args, "--spider") {
		return Result{Risk: Safe}
	}
	for i, a := range args {
		if a == "-O" || a == "--output-document" {
			if i+1 < len(args) && args[i+1] == "-" {
				return Result{Risk: Safe}
			}
		}
		if a == "-O-" || a == "--output-document=-" {
			return Result{Risk: Safe}
		}
	}
	return Result{Destructive, "wget saves the download to a file"}
}

func teeRule(args []string) Result {
	wrote := false
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		if isDiscardTarget(a) {
			continue
		}
		wrote = true
	}
	if wrote {
		return Result{Destructive, "tee writes its input to a file"}
	}
	return Result{Risk: Safe}
}

func tarRule(args []string) Result {
	if hasTarMode(args, 't') {
		return Result{Risk: Safe}
	}
	if hasTarMode(args, 'x') {
		return Result{Destructive, "tar -x extracts over existing files"}
	}
	if hasTarMode(args, 'c') || hasTarMode(args, 'r') || hasTarMode(args, 'u') {
		return Result{Destructive, "tar writes an archive file"}
	}
	if hasAnyFlag(args, "--list") {
		return Result{Risk: Safe}
	}
	if hasAnyFlag(args, "--extract", "--get") {
		return Result{Destructive, "tar -x extracts over existing files"}
	}
	if hasAnyFlag(args, "--create", "--append", "--update", "--delete") {
		return Result{Destructive, "tar writes an archive file"}
	}
	return Result{Unknown, "tar with no recognized mode flag"}
}

// hasTarMode looks for a mode letter in tar's bundled first-argument style
// (`tar -xzf`, `tar xzf`).
func hasTarMode(args []string, mode byte) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			continue
		}
		bundle := strings.TrimPrefix(a, "-")
		if bundle == "" || strings.ContainsAny(bundle, "/.") {
			continue
		}
		if strings.IndexByte(bundle, mode) >= 0 {
			return true
		}
	}
	return false
}

func compressRule(name string) func([]string) Result {
	return func(args []string) Result {
		if hasAnyFlag(args, "-c", "--stdout", "--to-stdout", "-l", "--list", "-t", "--test") {
			return Result{Risk: Safe}
		}
		return Result{Destructive, name + " replaces the original file with a compressed one"}
	}
}

func inPlaceRule(name string) func([]string) Result {
	return func(args []string) Result {
		if hasAnyFlag(args, "-i", "--in-place", "-i.bak") {
			return Result{Destructive, name + " -i rewrites the file in place"}
		}
		return Result{Risk: Safe}
	}
}

func scriptRunner(name string) func([]string) Result {
	return func(args []string) Result {
		return Result{Unknown, name + " runs an arbitrary script"}
	}
}

func hasAnyFlag(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f || strings.HasPrefix(a, f+"=") {
				return true
			}
		}
	}
	return false
}

// subTable classifies a command by its subcommand.
type subTable struct {
	safe        map[string]bool
	destructive map[string]string
	// groups are sub-namespaces to descend through: `docker volume rm` is
	// classified on "rm", not on "volume".
	groups map[string]bool
	// sub holds per-subcommand rules for subcommands that are themselves
	// flag-gated (`git branch` vs `git branch -D`).
	sub map[string]func(args []string) Result
}

func (t subTable) classify(name string, args []string) Result {
	sub, rest := "", args
	for {
		sub, rest = firstNonFlag(rest)
		if sub == "" || !t.groups[sub] {
			break
		}
	}
	if sub == "" {
		return Result{Unknown, name + " with no subcommand"}
	}
	if fn, ok := t.sub[sub]; ok {
		return fn(rest)
	}
	if reason, ok := t.destructive[sub]; ok {
		return Result{Destructive, reason}
	}
	if t.safe[sub] {
		return Result{Risk: Safe}
	}
	return Result{Unknown, "unrecognized " + name + " subcommand: " + sub}
}

// firstNonFlag returns the first argument that isn't an option, plus
// everything after it.
func firstNonFlag(args []string) (string, []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		return a, args[i+1:]
	}
	return "", nil
}

func flagGatedSub(safeFlags []string, destructiveFlags []string, reason string) func([]string) Result {
	return func(args []string) Result {
		if len(safeFlags) > 0 && hasAnyFlag(args, safeFlags...) {
			return Result{Risk: Safe}
		}
		if hasAnyFlag(args, destructiveFlags...) {
			return Result{Destructive, reason}
		}
		return Result{Risk: Safe}
	}
}

func subcommandSwitch(name string, safe map[string]bool, destructive map[string]string) func([]string) Result {
	return func(args []string) Result {
		sub, _ := firstNonFlag(args)
		if sub == "" {
			return Result{Unknown, name + " with no subcommand"}
		}
		if reason, ok := destructive[sub]; ok {
			return Result{Destructive, reason}
		}
		if safe[sub] {
			return Result{Risk: Safe}
		}
		return Result{Unknown, "unrecognized " + name + " subcommand: " + sub}
	}
}

var subcommandTables = map[string]subTable{
	"git": {
		safe: setOf("status", "log", "diff", "show", "describe", "rev-parse", "rev-list",
			"ls-files", "ls-remote", "ls-tree", "blame", "shortlog", "grep", "whatchanged",
			"cat-file", "fetch", "help", "version", "count-objects", "name-rev",
			"symbolic-ref", "verify-commit", "verify-tag", "fsck", "diff-tree", "add",
			"clone", "init", "bisect", "annotate", "check-ignore", "var", "for-each-ref"),
		destructive: map[string]string{
			"push":          "git push writes to the remote",
			"pull":          "git pull merges remote changes into your working tree",
			"reset":         "git reset moves refs and can discard changes",
			"clean":         "git clean deletes untracked files",
			"rm":            "git rm deletes tracked files",
			"mv":            "git mv renames tracked files",
			"checkout":      "git checkout overwrites files in your working tree",
			"switch":        "git switch overwrites files in your working tree",
			"restore":       "git restore overwrites files in your working tree",
			"commit":        "git commit changes repository history",
			"merge":         "git merge rewrites your working tree",
			"rebase":        "git rebase rewrites history",
			"revert":        "git revert adds a reverting commit",
			"cherry-pick":   "git cherry-pick applies commits to your branch",
			"am":            "git am applies patches to your branch",
			"apply":         "git apply modifies files in your working tree",
			"gc":            "git gc prunes unreachable objects",
			"prune":         "git prune deletes unreachable objects",
			"filter-branch": "git filter-branch rewrites every commit",
			"filter-repo":   "git filter-repo rewrites every commit",
			"update-ref":    "git update-ref moves a ref",
			"replace":       "git replace rewrites object references",
			"repack":        "git repack rewrites the object store",
		},
		sub: map[string]func([]string) Result{
			"branch": flagGatedSub(nil, []string{"-d", "-D", "--delete", "-m", "-M", "--move", "-c", "-C", "--copy", "--set-upstream-to", "-f", "--force"},
				"git branch is deleting or moving a branch"),
			"tag": flagGatedSub(nil, []string{"-d", "--delete", "-f", "--force"},
				"git tag is deleting or replacing a tag"),
			"stash": subcommandSwitch("git stash",
				setOf("list", "show"),
				map[string]string{
					"drop": "git stash drop discards a stash", "clear": "git stash clear discards every stash",
					"pop": "git stash pop rewrites your working tree", "apply": "git stash apply rewrites your working tree",
					"push":   "git stash push removes changes from your working tree",
					"save":   "git stash save removes changes from your working tree",
					"branch": "git stash branch rewrites your working tree",
				}),
			"worktree": subcommandSwitch("git worktree",
				setOf("list"),
				map[string]string{
					"remove": "git worktree remove deletes a worktree",
					"prune":  "git worktree prune deletes worktree metadata",
					"add":    "git worktree add creates a worktree on disk",
					"move":   "git worktree move relocates a worktree",
					"lock":   "git worktree lock changes worktree state",
					"unlock": "git worktree unlock changes worktree state",
				}),
			"submodule": subcommandSwitch("git submodule",
				setOf("status", "summary", "foreach"),
				map[string]string{
					"update": "git submodule update overwrites submodule checkouts",
					"add":    "git submodule add modifies the repository",
					"init":   "git submodule init modifies the repository config",
					"deinit": "git submodule deinit removes submodule checkouts",
					"sync":   "git submodule sync rewrites submodule config",
				}),
			"remote": subcommandSwitch("git remote",
				setOf("show", "get-url", "prune"),
				map[string]string{
					"add":      "git remote add changes repository config",
					"remove":   "git remote remove changes repository config",
					"rm":       "git remote rm changes repository config",
					"rename":   "git remote rename changes repository config",
					"set-url":  "git remote set-url changes repository config",
					"set-head": "git remote set-head changes repository config",
				}),
			"config": flagGatedSub([]string{"--get", "--get-all", "--get-regexp", "--list", "-l"},
				[]string{"--unset", "--unset-all", "--add", "--replace-all", "--edit", "-e"},
				"git config writes a config value"),
			"reflog": subcommandSwitch("git reflog",
				setOf("show", "exists"),
				map[string]string{"expire": "git reflog expire deletes reflog entries", "delete": "git reflog delete removes reflog entries"}),
		},
	},

	"docker": {
		groups: setOf("system", "volume", "network", "container", "image", "compose", "builder", "context", "plugin", "config", "secret", "node", "service", "stack", "swarm", "trust", "buildx"),
		safe: setOf("ps", "images", "logs", "inspect", "version", "info", "top", "port",
			"stats", "history", "search", "diff", "events", "ls", "list", "df", "login", "pull", "wait"),
		destructive: map[string]string{
			"rm": "docker rm deletes a container", "rmi": "docker rmi deletes an image",
			"kill": "docker kill stops a container immediately", "stop": "docker stop stops a container",
			"start": "docker start starts a container", "restart": "docker restart restarts a container",
			"pause": "docker pause suspends a container", "unpause": "docker unpause resumes a container",
			"prune": "docker prune deletes unused resources", "run": "docker run starts a new container",
			"exec": "docker exec runs a command inside a container", "create": "docker create creates a container",
			"build": "docker build creates or replaces an image tag", "push": "docker push writes to a registry",
			"tag": "docker tag reassigns an image tag", "commit": "docker commit creates an image from a container",
			"cp":     "docker cp copies files into or out of a container",
			"up":     "docker compose up creates and starts containers",
			"down":   "docker compose down stops and removes containers",
			"import": "docker import creates an image", "load": "docker load creates images",
			"save": "docker save writes an archive", "export": "docker export writes an archive",
			"update": "docker update changes container configuration",
			"rename": "docker rename renames a container",
		},
	},

	"kubectl": {
		safe: setOf("get", "describe", "logs", "explain", "top", "version", "api-resources",
			"api-versions", "cluster-info", "auth", "diff", "wait", "completion", "options"),
		destructive: map[string]string{
			"delete":    "kubectl delete removes cluster resources",
			"apply":     "kubectl apply changes cluster resources",
			"create":    "kubectl create adds cluster resources",
			"patch":     "kubectl patch changes cluster resources",
			"replace":   "kubectl replace overwrites cluster resources",
			"edit":      "kubectl edit changes cluster resources",
			"scale":     "kubectl scale changes replica counts",
			"drain":     "kubectl drain evicts pods from a node",
			"cordon":    "kubectl cordon changes node scheduling",
			"uncordon":  "kubectl uncordon changes node scheduling",
			"taint":     "kubectl taint changes node scheduling",
			"label":     "kubectl label changes resource metadata",
			"annotate":  "kubectl annotate changes resource metadata",
			"rollout":   "kubectl rollout changes workload state",
			"exec":      "kubectl exec runs a command inside a pod",
			"cp":        "kubectl cp copies files into or out of a pod",
			"run":       "kubectl run starts a new pod",
			"expose":    "kubectl expose creates a service",
			"set":       "kubectl set changes resource fields",
			"autoscale": "kubectl autoscale changes workload scaling",
			"attach":    "kubectl attach attaches to a running container",
		},
	},

	"systemctl": {
		safe: setOf("status", "list-units", "list-unit-files", "list-timers", "list-sockets",
			"show", "is-active", "is-enabled", "is-failed", "cat", "get-default", "help"),
		destructive: map[string]string{
			"start": "systemctl start starts a service", "stop": "systemctl stop stops a service",
			"restart": "systemctl restart restarts a service", "reload": "systemctl reload reloads a service",
			"enable": "systemctl enable changes boot configuration", "disable": "systemctl disable changes boot configuration",
			"mask": "systemctl mask disables a unit entirely", "unmask": "systemctl unmask re-enables a unit",
			"kill": "systemctl kill signals a service", "daemon-reload": "systemctl daemon-reload reloads unit definitions",
			"isolate": "systemctl isolate changes the running target", "set-default": "systemctl set-default changes boot configuration",
			"edit": "systemctl edit changes a unit definition", "revert": "systemctl revert discards unit overrides",
			"poweroff": "systemctl poweroff powers the machine down", "reboot": "systemctl reboot restarts the machine",
			"suspend": "systemctl suspend suspends the machine", "hibernate": "systemctl hibernate hibernates the machine",
		},
	},

	"launchctl": {
		safe: setOf("list", "print", "print-disabled", "help", "version", "blame"),
		destructive: map[string]string{
			"load": "launchctl load starts a service", "unload": "launchctl unload stops a service",
			"bootstrap": "launchctl bootstrap starts a service", "bootout": "launchctl bootout stops a service",
			"kickstart": "launchctl kickstart restarts a service", "remove": "launchctl remove removes a job",
			"enable": "launchctl enable changes service state", "disable": "launchctl disable changes service state",
			"stop": "launchctl stop stops a service", "start": "launchctl start starts a service",
			"kill": "launchctl kill signals a service",
		},
	},

	"defaults": {
		safe: setOf("read", "read-type", "domains", "find", "help"),
		destructive: map[string]string{
			"write": "defaults write changes a preference", "delete": "defaults delete removes a preference",
			"rename": "defaults rename changes a preference", "import": "defaults import overwrites preferences",
			"export": "defaults export writes a file",
		},
	},

	"npm":  packageManager("npm"),
	"pnpm": packageManager("pnpm"),
	"yarn": packageManager("yarn"),

	"brew": {
		groups: setOf("services"),
		safe: setOf("list", "ls", "info", "search", "outdated", "doctor", "config", "deps",
			"home", "leaves", "which", "desc", "cask", "help", "commands", "analytics"),
		destructive: map[string]string{
			"install": "brew install installs software", "uninstall": "brew uninstall removes software",
			"remove": "brew remove removes software", "rm": "brew rm removes software",
			"upgrade": "brew upgrade replaces installed versions", "update": "brew update rewrites its local repositories",
			"cleanup": "brew cleanup deletes old versions", "link": "brew link creates symlinks",
			"unlink": "brew unlink removes symlinks", "tap": "brew tap adds a repository",
			"untap": "brew untap removes a repository", "reinstall": "brew reinstall replaces an installation",
			"pin": "brew pin changes upgrade behavior", "unpin": "brew unpin changes upgrade behavior",
			"autoremove": "brew autoremove uninstalls unused dependencies",
			"start":      "brew services start starts a service", "stop": "brew services stop stops a service",
			"restart": "brew services restart restarts a service",
		},
	},

	"apt":     systemPackageManager("apt"),
	"apt-get": systemPackageManager("apt-get"),
	"dnf":     systemPackageManager("dnf"),
	"yum":     systemPackageManager("yum"),
	"zypper":  systemPackageManager("zypper"),
	"apk":     systemPackageManager("apk"),

	"pip":  pipManager("pip"),
	"pip3": pipManager("pip3"),

	"gem": {
		safe: setOf("list", "search", "info", "which", "environment", "help", "outdated", "contents"),
		destructive: map[string]string{
			"install": "gem install installs software", "uninstall": "gem uninstall removes software",
			"update": "gem update replaces installed versions", "cleanup": "gem cleanup deletes old versions",
			"push": "gem push publishes a gem", "yank": "gem yank removes a published gem",
		},
	},

	"cargo": {
		safe: setOf("tree", "search", "metadata", "version", "help", "locate-project", "pkgid", "verify-project"),
		destructive: map[string]string{
			"install": "cargo install installs a binary", "uninstall": "cargo uninstall removes a binary",
			"publish": "cargo publish publishes a crate", "clean": "cargo clean deletes build output",
			"fix": "cargo fix rewrites your source files", "fmt": "cargo fmt rewrites your source files",
			"update": "cargo update rewrites Cargo.lock", "add": "cargo add rewrites Cargo.toml",
			"remove": "cargo remove rewrites Cargo.toml", "yank": "cargo yank removes a published version",
		},
		sub: map[string]func([]string) Result{
			"build": projectCode("cargo build"), "test": projectCode("cargo test"),
			"run": projectCode("cargo run"), "bench": projectCode("cargo bench"),
			"check": projectCode("cargo check"),
		},
	},

	"go": {
		safe: setOf("version", "env", "doc", "list", "vet", "help", "tool"),
		destructive: map[string]string{
			"clean": "go clean deletes build output", "fmt": "go fmt rewrites your source files",
			"install": "go install writes a binary into GOBIN", "work": "go work rewrites go.work",
			"get": "go get rewrites go.mod",
		},
		sub: map[string]func([]string) Result{
			"mod": subcommandSwitch("go mod",
				setOf("why", "graph", "download", "verify"),
				map[string]string{
					"tidy":   "go mod tidy rewrites go.mod and go.sum",
					"vendor": "go mod vendor rewrites the vendor directory",
					"edit":   "go mod edit rewrites go.mod", "init": "go mod init writes go.mod",
				}),
			"build": projectCode("go build"), "test": projectCode("go test"),
			"run": projectCode("go run"), "generate": projectCode("go generate"),
		},
	},

	"terraform": {
		safe: setOf("plan", "show", "validate", "output", "version", "providers", "graph", "console", "login", "logout"),
		destructive: map[string]string{
			"apply":   "terraform apply changes real infrastructure",
			"destroy": "terraform destroy tears down real infrastructure",
			"import":  "terraform import rewrites state", "taint": "terraform taint rewrites state",
			"untaint": "terraform untaint rewrites state", "state": "terraform state rewrites state",
			"workspace":    "terraform workspace changes or deletes workspaces",
			"init":         "terraform init writes provider plugins and state config",
			"fmt":          "terraform fmt rewrites your source files",
			"refresh":      "terraform refresh rewrites state",
			"force-unlock": "terraform force-unlock releases a state lock",
		},
	},

	"helm": {
		safe: setOf("list", "ls", "get", "status", "show", "search", "history", "version", "template", "lint", "env"),
		destructive: map[string]string{
			"install": "helm install deploys a release", "upgrade": "helm upgrade changes a release",
			"uninstall": "helm uninstall removes a release", "delete": "helm delete removes a release",
			"rollback": "helm rollback changes a release", "repo": "helm repo changes configured repositories",
			"dependency": "helm dependency writes chart dependencies", "push": "helm push writes to a registry",
			"create": "helm create writes a new chart on disk",
		},
	},
}

func packageManager(name string) subTable {
	return subTable{
		safe: setOf("ls", "list", "view", "info", "outdated", "why", "search", "audit",
			"doctor", "ping", "whoami", "config", "help", "root", "bin", "explain", "licenses"),
		destructive: map[string]string{
			"install": name + " install writes to node_modules and the lockfile",
			"i":       name + " i writes to node_modules and the lockfile",
			"add":     name + " add writes to node_modules and the lockfile",
			"ci":      name + " ci deletes and reinstalls node_modules",
			"remove":  name + " remove uninstalls packages", "rm": name + " rm uninstalls packages",
			"uninstall": name + " uninstall removes packages", "un": name + " un removes packages",
			"update": name + " update replaces installed versions", "up": name + " up replaces installed versions",
			"upgrade": name + " upgrade replaces installed versions",
			"prune":   name + " prune deletes packages", "dedupe": name + " dedupe rewrites node_modules",
			"link": name + " link creates global symlinks", "publish": name + " publish publishes to a registry",
			"unpublish": name + " unpublish removes a published version",
			"version":   name + " version bumps the version and tags a commit",
			"deprecate": name + " deprecate changes a published package",
			"cache":     name + " cache can delete the package cache",
			"init":      name + " init writes package.json",
		},
		sub: map[string]func([]string) Result{
			"run":        projectCode(name + " run"),
			"run-script": projectCode(name + " run-script"),
			"exec":       projectCode(name + " exec"),
			"start":      projectCode(name + " start"),
			"test":       projectCode(name + " test"),
			"build":      projectCode(name + " build"),
			"dlx":        projectCode(name + " dlx"),
		},
	}
}

func systemPackageManager(name string) subTable {
	return subTable{
		safe: setOf("list", "show", "search", "info", "policy", "depends", "rdepends", "help", "changelog", "madison"),
		destructive: map[string]string{
			"install":      name + " install installs system packages",
			"remove":       name + " remove uninstalls system packages",
			"purge":        name + " purge uninstalls packages and their config",
			"upgrade":      name + " upgrade replaces installed packages",
			"dist-upgrade": name + " dist-upgrade replaces installed packages",
			"full-upgrade": name + " full-upgrade replaces installed packages",
			"autoremove":   name + " autoremove uninstalls packages",
			"update":       name + " update rewrites the local package index",
			"clean":        name + " clean deletes the package cache",
			"reinstall":    name + " reinstall replaces an installed package",
			"add":          name + " add installs system packages",
			"del":          name + " del uninstalls system packages",
		},
	}
}

func pipManager(name string) subTable {
	return subTable{
		safe: setOf("list", "show", "freeze", "search", "check", "config", "help", "debug", "cache"),
		destructive: map[string]string{
			"install":   name + " install installs packages into your environment",
			"uninstall": name + " uninstall removes packages",
			"download":  name + " download writes package files to disk",
			"wheel":     name + " wheel writes built wheels to disk",
		},
	}
}

// projectCode marks the "run whatever this project defines" subcommands.
// They aren't destructive by name, but nothing here can see what they'll
// actually do, so they land on Unknown and get confirmed.
func projectCode(label string) func([]string) Result {
	return func(args []string) Result {
		return Result{Unknown, label + " runs code from this project"}
	}
}

// safeCommands is the read-only / purely-additive allowlist: nothing here
// may delete, overwrite or truncate existing data, or change system,
// container or remote state. Everything else falls through to Unknown.
var safeCommands = setOf(
	// Listing and navigating.
	"ls", "dir", "vdir", "tree", "pwd", "cd", "pushd", "popd", "dirs", "basename",
	"dirname", "realpath", "readlink", "locate", "mdfind",
	// Reading files.
	"cat", "bat", "tac", "nl", "head", "tail", "less", "more", "most", "view",
	"strings", "xxd", "od", "hexdump", "file", "stat", "wc", "du", "df",
	// Text processing that writes only to stdout.
	"sort", "uniq", "cut", "paste", "join", "column", "tr", "rev", "fold",
	"expand", "unexpand", "comm", "diff", "cmp", "sdiff", "diffstat", "fmt",
	"grep", "egrep", "fgrep", "rg", "ag", "ack", "ugrep", "awk", "gawk", "mawk",
	"look", "pr", "csplit",
	// Output.
	"echo", "printf", "yes", "seq", "true", "false", "test", "expr", "bc", "dc",
	"sleep", "clear", "tput", "pbcopy", "pbpaste", "say", "banner", "figlet",
	// Creating without destroying.
	"mkdir", "touch", "mktemp", "mkfifo",
	// Identity and environment.
	"whoami", "id", "groups", "users", "w", "who", "hostname", "uname", "arch",
	"date", "cal", "uptime", "printenv", "export", "alias", "unalias", "set",
	"unset", "which", "whereis", "type", "hash", "locale", "tty", "stty", "logname",
	// Processes (observation only).
	"ps", "pgrep", "top", "htop", "btop", "atop", "free", "vmstat", "iostat",
	"lsof", "jobs", "fg", "bg", "wait", "nproc", "sysctl", "pmap",
	// Networking (observation only).
	"ping", "ping6", "traceroute", "traceroute6", "dig", "nslookup", "host",
	"whois", "netstat", "ss", "arp", "ipconfig", "scutil",
	// Checksums and encoding.
	"md5", "md5sum", "shasum", "sha1sum", "sha256sum", "sha512sum", "cksum",
	"base64", "uuidgen",
	// Documentation.
	"man", "info", "help", "tldr", "apropos", "whatis",
	// Misc read-only tooling.
	"open", "code", "shellcheck",
)

func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
