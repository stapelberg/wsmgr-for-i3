package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"

	"go.i3wm.org/i3/v4"
)

var nameRe = regexp.MustCompile(`^\d+: (.*)`)

func getWorkspaceName() string {
	tree, err := i3.GetTree()
	if err != nil {
		log.Fatal(err)
	}

	ws := tree.Root.FindFocused(func(n *i3.Node) bool { return n.Type == i3.WorkspaceNode })
	if ws == nil {
		log.Fatal("could not locate workspace")
	}
	name := ws.Name
	if matches := nameRe.FindStringSubmatch(name); len(matches) > 1 {
		name = matches[1]
	}
	return name
}

func chdir() error {
	// get the current workspace’s name
	workspaceName := getWorkspaceName()
	log.Printf("workspace name = %q", workspaceName)

	// check if ~/.config/wsmgr-for-i3/<name>/cwd exists
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	target, err := os.Readlink(filepath.Join(configDir, "wsmgr-for-i3", workspaceName, "cwd"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.Chdir(target); err != nil {
		return err
	}
	return nil
}

func cwd() error {
	if err := chdir(); err != nil {
		log.Print(err)
	}

	// check if we have something to run
	if len(os.Args[1:]) < 2 {
		log.Fatal("no command to execute")
	}

	// run the remaining command line args
	args := os.Args[1:]
	full, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(full, args, os.Environ())
}

func main() {
	if err := cwd(); err != nil {
		log.Fatal(err)
	}
}
