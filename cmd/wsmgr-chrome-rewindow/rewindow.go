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

func openNewWindow(children []Bookmark) {
	chrome := exec.Command("google-chrome", "--new-window")
	for _, ch := range children {
		chrome.Args = append(chrome.Args, ch.URL)
	}
	log.Printf("%v", chrome)
	chrome.Stdout = os.Stdout
	chrome.Stderr = os.Stderr
	chrome.Stdin = os.Stdin
	if err := chrome.Run(); err != nil {
		log.Fatal(err)
	}
}

func rewindow() error {
	var (
		list = flag.Bool("list", false, "list bookmark folder names")
		name = flag.String("name", "", "name of the bookmark folder to open in a new window")
	)
	flag.Parse()
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(filepath.Join(configDir, "google-chrome/Default/Bookmarks"))
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
			log.Printf("opening folder %q in new chrome window", b.Name)
			openNewWindow(b.Children)
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
