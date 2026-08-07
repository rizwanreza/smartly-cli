package classify

import (
	"strings"
	"testing"
)

// check asserts the verdict for a command and enforces the package-wide
// invariant that any non-Safe result carries an explanation (it is rendered
// straight into the confirmation prompt).
func check(t *testing.T, command string, want Risk) {
	t.Helper()
	got := Classify(command)
	if got.Risk != want {
		t.Errorf("Classify(%q).Risk = %v (%q), want %v", command, got.Risk, got.Reason, want)
	}
	if got.Risk != Safe && strings.TrimSpace(got.Reason) == "" {
		t.Errorf("Classify(%q) returned %v with an empty Reason", command, got.Risk)
	}
	if got.Risk == Safe && got.Reason != "" {
		t.Errorf("Classify(%q) returned Safe with Reason %q, want no reason", command, got.Reason)
	}
}

func runTable(t *testing.T, want Risk, commands []string) {
	t.Helper()
	for _, c := range commands {
		t.Run(c, func(t *testing.T) { check(t, c, want) })
	}
}

func TestRiskString(t *testing.T) {
	tests := []struct {
		risk Risk
		want string
	}{
		{Safe, "safe"},
		{Destructive, "destructive"},
		{Unknown, "unknown"},
		{Risk(99), "unknown"}, // totality: an out-of-range value is never "safe"
	}
	for _, tt := range tests {
		if got := tt.risk.String(); got != tt.want {
			t.Errorf("Risk(%d).String() = %q, want %q", tt.risk, got, tt.want)
		}
	}
}

func TestNeedsConfirm(t *testing.T) {
	safe := Result{Risk: Safe}
	unknown := Result{Risk: Unknown, Reason: "x"}
	destructive := Result{Risk: Destructive, Reason: "x"}

	if safe.NeedsConfirm() {
		t.Error("Safe should not need confirmation")
	}
	if !unknown.NeedsConfirm() {
		t.Error("Unknown must need confirmation — an unrecognized command is not known-safe")
	}
	if !destructive.NeedsConfirm() {
		t.Error("Destructive must need confirmation")
	}
}

func TestClassify_Safe(t *testing.T) {
	runTable(t, Safe, []string{
		"ls",
		"ls -la",
		"ls -la /tmp",
		"cat README.md",
		"head -n 20 log.txt",
		"tail -n 100 development.log",
		"grep -rn TODO .",
		"rg --hidden pattern src/",
		"wc -l *.go",
		"du -sh .",
		"df -h",
		"ps aux",
		"pwd",
		"cd /tmp",
		"echo hello",
		"date",
		"uname -a",
		"which git",
		"file /bin/sh",
		"sort names.txt | uniq -c",
		"cat access.log | grep 500 | wc -l",
		"mkdir -p build/output",
		"touch newfile.txt",
		"sed 's/git/svn/g' file.txt",
		"find . -type f -name '*.log'",
		"find . -name '*.go' -print",
		"curl https://example.com",
		"curl -s -H 'Accept: application/json' https://example.com",
		"curl -X GET https://example.com",
		"wget --spider https://example.com",
		"wget -O - https://example.com",
		"tar -tzf archive.tar.gz",
		"git status",
		"git log --oneline -10",
		"git diff HEAD~1",
		"git branch",
		"git branch -a",
		"git tag",
		"git stash list",
		"git worktree list",
		"git fetch origin",
		"git config --get user.email",
		"git remote show origin",
		"docker ps -a",
		"docker images",
		"docker volume ls",
		"docker logs mycontainer",
		"kubectl get pods -A",
		"kubectl describe pod foo",
		"systemctl status nginx",
		"brew list",
		"npm ls --depth=0",
		"pip list",
		"terraform plan",
		"helm list",
		"go version",
		"go mod why example.com/x",
		"crontab -l",
		"jq '.items[]' data.json",
		"defaults read com.apple.dock",
		"launchctl list",
		"ls > /dev/null",
		"ls > /dev/null 2>&1",
		"ls 2>/dev/null",
		"grep foo bar.txt 2>&1",
		"ls < input.txt",
		"echo hi | tee /dev/null",
		"history",
	})
}

func TestClassify_Destructive(t *testing.T) {
	runTable(t, Destructive, []string{
		"rm -rf ./build",
		"rm file.txt",
		"/bin/rm -rf /tmp/x",
		"\\rm -rf /tmp/x", // backslash suppresses aliases; it must not hide rm
		"rmdir olddir",
		"dd if=/dev/zero of=/dev/disk2 bs=1m",
		"mkfs.ext4 /dev/sda1",
		"shred -u secrets.txt",
		"mv old.txt new.txt",
		"cp -r src dst",
		"rsync -av --delete src/ dst/",
		"ln -sf a b",
		"chmod 777 script.sh",
		"chown -R me:staff .",
		"truncate -s 0 big.log",
		"kill -9 1234",
		"pkill -f node",
		"mount /dev/disk2 /mnt",
		"sudo ls",
		"sudo systemctl restart nginx",
		"sed -i '' 's/git/svn/g' file.txt",
		"sed -i.bak 's/a/b/' file.txt",
		"sed --in-place 's/a/b/' file.txt",
		"find . -name '*.log' -delete",
		"find . -type f -exec rm {} +",
		"find . -type f -exec sed -i '' 's/git/svn/g' {} +",
		"find . -name '*.log' | xargs rm",
		"find . -name '*.tmp' -print0 | xargs -0 rm -f",
		"curl -o out.html https://example.com",
		"curl -O https://example.com/file.zip",
		"curl -X DELETE https://api.example.com/things/1",
		"curl -d 'a=1' https://api.example.com",
		"wget https://example.com/file.zip",
		"tar -xzf archive.tar.gz",
		"tar -czf archive.tar.gz src/",
		"unzip archive.zip",
		"gzip bigfile.log",
		"tee output.txt",
		"echo hi | tee -a output.txt",
		"echo hello > out.txt",
		"echo hello >> out.txt",
		"cat a.txt > b.txt",
		"ls >| forced.txt",
		"ls &> combined.log",
		"git push --force origin main",
		"git reset --hard HEAD~3",
		"git clean -fd",
		"git checkout .",
		"git commit -m 'wip'",
		"git rebase -i main",
		"git branch -D feature",
		"git tag -d v1.0.0",
		"git stash drop",
		"git stash pop",
		"git worktree remove /Users/you/project-fix",
		"git remote add origin git@github.com:x/y.git",
		"git config --unset user.email",
		"git submodule update --init",
		"git reflog expire --expire=now --all",
		"docker rm -f mycontainer",
		"docker rmi myimage",
		"docker system prune -af",
		"docker volume rm myvolume",
		"docker compose down",
		"docker exec -it web sh",
		"kubectl delete pod foo",
		"kubectl apply -f deploy.yaml",
		"kubectl exec -it web -- sh",
		"systemctl stop nginx",
		"systemctl daemon-reload",
		"brew install ripgrep",
		"brew services restart postgresql",
		"npm install express",
		"npm ci",
		"yarn add lodash",
		"pip install requests",
		"apt-get install -y curl",
		"terraform apply -auto-approve",
		"terraform destroy",
		"helm uninstall myrelease",
		"go mod tidy",
		"go fmt ./...",
		"cargo install ripgrep",
		"gem uninstall rails",
		"crontab -r",
		"history -c",
		"jq -i '.a = 1' data.json",
		"defaults write com.apple.dock autohide -bool true",
		"launchctl unload ~/Library/LaunchAgents/x.plist",
		"pacman -Rns package",
		"timeout 30 rm -rf /tmp/cache",
		"nohup rm -rf /tmp/cache",
		"env FOO=bar rm -rf /tmp/cache",
		"nice -n 10 rm -rf /tmp/cache",
	})
}

func TestClassify_Unknown(t *testing.T) {
	runTable(t, Unknown, []string{
		"frobnicate --all",
		"./deploy.sh",
		"bash deploy.sh",
		"sh -c 'echo hi'", // recognized inner command, but a shell string still asks
		"make build",
		"ssh host 'uptime'",
		"python script.py",
		"node server.js",
		"source ~/.bashrc",
		". ./env.sh",
		"git frobnicate",
		"docker frobnicate",
		"kubectl frobnicate",
		"npm run build",
		"go test ./...",
		"cargo build --release",
		"eval $CMD",
		"echo $(hostname)",
		"echo `hostname`",
		"ls $(cat dirs.txt)",
		"diff <(sort a.txt) <(sort b.txt)",
		"vim notes.txt",
		"perl -pe 's/a/b/' file.txt",
		"tar --frobnicate archive.tar",
	})
}

// Quoted text must never trigger a rule: the tokenizer is what makes
// `echo "rm -rf /"` different from `rm -rf /`.
func TestClassify_QuotedTextIsNotATrigger(t *testing.T) {
	runTable(t, Safe, []string{
		`echo "rm -rf /"`,
		`echo 'sudo shutdown now'`,
		`echo "dd if=/dev/zero of=/dev/disk0"`,
		`grep "rm -rf" history.txt`,
		`grep -r 'chmod 777' .`,
		`echo "a > b"`,         // a quoted > is not a redirect
		`echo "a | rm -rf /"`,  // a quoted | is not a segment break
		`echo "a && rm -rf /"`, // nor is a quoted &&
		`echo "a; rm -rf /"`,   // nor a quoted ;
		`echo 'nested "rm -rf" quotes'`,
		`printf '%s\n' "| sudo rm"`,
		`echo '$(rm -rf /)'`, // single quotes suppress substitution entirely
		`grep -F "2>&1" logs.txt`,
	})

	// The same strings unquoted are exactly what the classifier is for.
	runTable(t, Destructive, []string{
		`rm -rf /tmp/x`,
		`sudo shutdown now`,
		`echo a > b`,
	})
}

// A command's risk is the maximum risk of its segments, however they are
// joined.
func TestClassify_SegmentAggregation(t *testing.T) {
	runTable(t, Destructive, []string{
		"ls | xargs rm",
		"cat list.txt | xargs -n1 rm -f",
		"ls && rm -rf build",
		"ls || rm -rf build",
		"ls; rm -rf build",
		"rm -rf build; ls",
		"ls & rm -rf build",
		"cat a.txt | sort | uniq > out.txt",
		"grep -l TODO *.go | xargs sed -i '' 's/TODO/DONE/'",
		"(cd /tmp && rm -rf cache)",
		"for f in *.log; do rm $f; done",
		"if [ -d build ]; then rm -rf build; fi",
	})

	runTable(t, Unknown, []string{
		"ls | frobnicate",
		"frobnicate && ls",
		"ls; ./script.sh",
	})

	runTable(t, Safe, []string{
		"ls | grep foo",
		"cat a.txt | head -5 | wc -l",
		"ls && pwd && date",
	})

	// Destructive outranks Unknown even when the unknown segment comes first.
	check(t, "frobnicate | rm -rf x", Destructive)
	check(t, "rm -rf x | frobnicate", Destructive)
}

func TestClassify_SudoEvalAndSubstitution(t *testing.T) {
	tests := []struct {
		command string
		want    Risk
		reason  string
	}{
		// sudo is unconditional — escalation is itself the thing to confirm.
		{"sudo ls", Destructive, "sudo runs the command with root privileges"},
		{"sudo -u nobody ls", Destructive, "sudo runs the command with root privileges"},
		{"doas ls", Destructive, "sudo runs the command with root privileges"},

		// eval floors at Unknown, but a recognizably destructive body wins.
		{"eval $CMD", Unknown, "eval runs a command assembled at runtime"},
		{"eval 'ls -la'", Unknown, "eval runs a command assembled at runtime"},
		{"eval 'rm -rf /tmp/x'", Destructive, "rm deletes files"},

		// Substitutions float to at least Unknown even around a safe command.
		{"echo $(date)", Unknown, "command substitution can run anything"},
		{"echo `date`", Unknown, "command substitution can run anything"},
		// …and a destructive body inside a substitution still wins.
		{"echo $(rm -rf /tmp/x)", Destructive, "rm deletes files"},
		{"echo \"$(rm -rf /tmp/x)\"", Destructive, "rm deletes files"},

		// Arithmetic expansion is not a command substitution.
		{"echo $((1 + 2))", Safe, ""},

		// sh -c floors at Unknown; a destructive script still wins.
		{"sh -c 'rm -rf /tmp/x'", Destructive, "rm deletes files"},
		{"bash -c 'ls'", Unknown, "bash -c runs a command string"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := Classify(tt.command)
			if got.Risk != tt.want {
				t.Errorf("Classify(%q).Risk = %v, want %v (reason %q)", tt.command, got.Risk, tt.want, got.Reason)
			}
			if got.Reason != tt.reason {
				t.Errorf("Classify(%q).Reason = %q, want %q", tt.command, got.Reason, tt.reason)
			}
		})
	}
}

func TestClassify_RedirectCarveOuts(t *testing.T) {
	tests := []struct {
		command string
		want    Risk
	}{
		{"ls > out.txt", Destructive},
		{"ls >> out.txt", Destructive},
		{"ls >| out.txt", Destructive},
		{"ls &> out.txt", Destructive},
		{"ls &>> out.txt", Destructive},
		{"ls >&out.txt", Destructive}, // bash shorthand for &>
		{"ls > /dev/null", Safe},
		{"ls 2> /dev/null", Safe},
		{"ls 2>/dev/null", Safe},
		{"ls > /dev/null 2>&1", Safe}, // 2>&1 is an fd dup, not a write
		{"ls >&2", Safe},
		{"ls >&-", Safe},
		{"ls > /dev/stdout", Safe},
		{"ls > /dev/fd/3", Safe},
		{"sort < input.txt", Safe},
		{"cat <<< 'here string'", Safe},
		{"wc -l < big.log", Safe},
		{"grep foo bar.txt > /dev/null && echo found", Safe},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) { check(t, tt.command, tt.want) })
	}
}

// Classify must be total: every input produces a verdict, no panics, and
// nothing malformed is ever silently promoted to Safe when it contains a
// recognizably destructive command.
func TestClassify_AdversarialInputTotality(t *testing.T) {
	inputs := []string{
		"",
		" ",
		"\t\t",
		"|",
		"||",
		"&&",
		";;;",
		"&",
		">",
		">>",
		"<",
		"2>",
		"$(",
		"$((",
		"`",
		"'",
		`"`,
		`"unterminated`,
		`'unterminated`,
		"$(unterminated",
		"`unterminated",
		"((((((((((",
		"))))))))))",
		"\\",
		"\\\\",
		"rm -rf '",
		`rm -rf "`,
		"echo \x00 hi",
		"echo \x1b[31m",
		"# just a comment",
		"echo hi # trailing comment",
		strings.Repeat("a", 10000),
		strings.Repeat("$(", 200) + strings.Repeat(")", 200),
		strings.Repeat("rm -rf x | ", 500) + "ls",
		strings.Repeat("| ", 1000),
		"éèê",         // multi-byte runes
		"echo \"你好\"", // multi-byte inside quotes
		"rm -rf /",    // non-breaking spaces are not separators
	}

	for _, in := range inputs {
		name := in
		if len(name) > 40 {
			name = name[:40] + "…"
		}
		t.Run(name, func(t *testing.T) {
			got := Classify(in)
			switch got.Risk {
			case Safe, Unknown, Destructive:
			default:
				t.Fatalf("Classify(%q) returned out-of-range risk %d", in, got.Risk)
			}
			if got.Risk != Safe && got.Reason == "" {
				t.Errorf("Classify(%q) = %v with no reason", in, got.Risk)
			}
		})
	}

	// Unterminated quoting must not let a destructive command through.
	for _, in := range []string{"rm -rf '", `rm -rf "`, "rm -rf /tmp/x |"} {
		if got := Classify(in); got.Risk != Destructive {
			t.Errorf("Classify(%q) = %v, want Destructive even though the input is malformed", in, got.Risk)
		}
	}

	// Deep nesting degrades to Unknown (ask), never to Safe (run).
	deep := strings.Repeat("$(", 50) + "ls" + strings.Repeat(")", 50)
	if got := Classify(deep); got.Risk == Safe {
		t.Errorf("Classify(deeply nested substitution) = Safe, want a confirming verdict")
	}
}

func TestClassify_PathAndPrefixStripping(t *testing.T) {
	tests := []struct {
		command string
		want    Risk
	}{
		{"/bin/rm -rf /tmp/x", Destructive},
		{"/usr/bin/env rm -rf /tmp/x", Destructive},
		{"FOO=bar BAZ=qux rm -rf /tmp/x", Destructive},
		{"FOO=bar ls", Safe},
		{"command rm -rf /tmp/x", Destructive},
		{"time ls", Safe},
		{"xargs echo", Safe},
		{"xargs", Safe},
		{"xargs -I{} rm -rf {}", Destructive},
		{"xargs -0 -n 1 grep foo", Safe},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) { check(t, tt.command, tt.want) })
	}
}

func FuzzClassify(f *testing.F) {
	seeds := []string{
		"", "ls -la", "rm -rf ./build", "echo \"rm -rf /\"",
		"find . -name '*.log' | xargs rm", "frobnicate --all",
		"sudo rm -rf /", "echo hi > out.txt", "ls > /dev/null 2>&1",
		"eval $(cat cmd.txt)", "git worktree remove x", "$(", "`", "'", "\"",
		"a|b&&c;d&e", "\\", "$((1+2))", "<(ls)", "2>&1",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, command string) {
		// Totality: no panic, a valid tri-state, and a reason whenever the
		// verdict would stop and ask.
		got := Classify(command)
		switch got.Risk {
		case Safe:
			if got.Reason != "" {
				t.Fatalf("Classify(%q) = Safe but carried reason %q", command, got.Reason)
			}
		case Unknown, Destructive:
			if got.Reason == "" {
				t.Fatalf("Classify(%q) = %v with no reason", command, got.Risk)
			}
		default:
			t.Fatalf("Classify(%q) returned out-of-range risk %d", command, got.Risk)
		}

		// Determinism: the same input must always classify the same way.
		if again := Classify(command); again != got {
			t.Fatalf("Classify(%q) is not deterministic: %+v then %+v", command, got, again)
		}
	})
}
