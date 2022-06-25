package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type Bookmark struct {
	Guid     string     `json:"guid"`
	Name     string     `json:"name"`
	URL      string     `json:"url"`
	Type     string     `json:"type"`
	Children []Bookmark `json:"children"`
}

type bookmarkRoot struct {
	Children []Bookmark `json:"children"`
}

type bookmarkFile struct {
	Roots map[string]bookmarkRoot `json:"roots"`
}

type Browser struct {
	Executable string
	ConfigDir  string
	FlatPak    string
	flat       bool
}

var browsers = map[string]Browser{
	"brave":    {Executable: "brave-browser", ConfigDir: "BraveSoftware/Brave-Browser/Default/Bookmarks", FlatPak: "com.brave.Browser"},
	"chrome":   {Executable: "google-chrome", ConfigDir: "google-chrome/Default/Bookmarks", FlatPak: "com.google.Chrome"},
	"chromium": {Executable: "chromium-browser", ConfigDir: "chromium/Default/Bookmarks", FlatPak: "org.chromium.Chromium"},
	"edge":     {Executable: "msedge", ConfigDir: "microsoft-edge/Default/Bookmarks", FlatPak: "com.microsoft.Edge"},
}

func inspectChildren(children []Bookmark, f func(Bookmark) bool) bool {
	for _, ch := range children {
		if !f(ch) {
			return false
		}
		if ch.Type == "folder" {
			if !inspectChildren(ch.Children, f) {
				return false
			}
		}
	}
	return true
}

func inspect(root bookmarkRoot, f func(Bookmark) bool) {
	inspectChildren(root.Children, f)
}

func (b *Browser) openNewWindow(children []Bookmark) {
	var cmd *exec.Cmd
	if b.flat {
		cmd = exec.Command("flatpak", "run", b.FlatPak, "--new-window")
	} else {
		cmd = exec.Command(b.Executable, "--new-window")
	}
	for _, ch := range children {
		cmd.Args = append(cmd.Args, ch.URL)
	}
	log.Printf("%v", cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func (b *Browser) readFile() ([]byte, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(configDir, b.ConfigDir))
	if err == nil {
		return bytes, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	b.flat = true

	return ioutil.ReadFile(filepath.Join(homeDir, ".var/app", b.FlatPak, "config", b.ConfigDir))
}

func rewindow() error {
	var (
		list    = flag.Bool("list", false, "list bookmark folder names")
		browser = flag.String("browser", "google-chrome", "your preferred chromium flavour (default google-chrome)")
		name    = flag.String("name", "", "name of the bookmark folder to open in a new window")
	)
	flag.Parse()

	config, ok := browsers[*browser]
	if !ok {
		log.Printf("Supported browsers:")
		for browser := range browsers {
			log.Printf("  - %s", browser)
		}
		log.Println()
		log.Fatalf("Browser '%s' not supported (yet, feel free to create a PR)", *browser)
	}

	b, err := config.readFile()
	if err != nil {
		return err
	}
	var a bookmarkFile
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	if *list {
		var folderNames []string
		inspect(a.Roots["bookmark_bar"], func(b Bookmark) bool {
			if b.Type == "folder" {
				folderNames = append(folderNames, b.Name)
			}
			return true
		})
		for _, name := range folderNames {
			fmt.Println(name)
		}
		return nil
	}

	if *name == "" {
		return fmt.Errorf("neither -list nor -name specified")
	}

	inspect(a.Roots["bookmark_bar"], func(b Bookmark) bool {
		if b.Type == "folder" && b.Name == *name {
			log.Printf("opening folder %q in new %s window", b.Name, *browser)
			config.openNewWindow(b.Children)
			return false
		}
		return true
	})

	return nil
}

func main() {
	if err := rewindow(); err != nil {
		log.Fatal(err)
	}
}
