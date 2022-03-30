// The whole program uses log.Fatal for error handling, under the assumption
// that any error is either a bug (likely with how we use GTK, or in gotk3), or
// i3 having gone away, in which case this program should terminate, too.
//
// Workspaces are configured in ~/.config/wsmgr-for-i3/<name>. Each setting is
// configured via its own file.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/renameio/v2"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/spf13/cobra"
	"go.i3wm.org/i3/v4"

	_ "embed"
)

type wsmgr struct {
	currentWorkspace struct {
		store        *gtk.ListStore
		tv           *gtk.TreeView
		ignoreEvents bool
	}

	addWorkspaceButton *gtk.Button

	workspaceLoaderTV *gtk.TreeView
}

func updateConfiguredWorkspaces(store *gtk.ListStore) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	fis, err := ioutil.ReadDir(filepath.Join(configDir, "wsmgr-for-i3"))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Fatal(err)
	}
	for _, fi := range fis {
		if !fi.Mode().IsDir() {
			continue
		}
		if fi.Name() == "." || fi.Name() == ".." {
			continue
		}
		store.Set(store.Append(), []int{0, 1}, []interface{}{fi.Name(), 0})
	}
}

func (w *wsmgr) loadWorkspace(name string) {
	log.Printf("Loading workspace %q", name)
	w.addWorkspace(name)

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(configDir, "wsmgr-for-i3", name)
	cwd, err := filepath.EvalSymlinks(filepath.Join(dir, "cwd"))
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, fi := range fis {
		if fi.Mode().IsDir() && (fi.Name() == "." || fi.Name() == "..") {
			continue
		}

		if fi.Name() == "cwd" {
			continue
		}

		path := filepath.Join(dir, fi.Name())

		executable := fi.Mode()&0100 != 0
		symlink := fi.Mode()&os.ModeSymlink != 0
		if symlink {
			fi, err := os.Stat(path)
			if err != nil {
				log.Print(err)
				continue
			}
			executable = fi.Mode()&0100 != 0
			if fi.Mode().IsDir() {
				executable = false
			}
		}
		if executable {
			log.Printf("starting executable %s", path)
			// File executable by its owner, try to execute it
			cmd := exec.Command(path)
			if cwd != "" {
				cmd.Dir = cwd
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			go func() {
				if err := cmd.Run(); err != nil {
					log.Printf("%v: %v", cmd.Args, err)
				}
			}()
		}

		if fi.Name() == "chrome-rewindow" {
			b, err := ioutil.ReadFile(path)
			if err != nil {
				log.Print(err)
				continue
			}
			n := strings.TrimSpace(string(b))
			cmd := exec.Command("wsmgr-chrome-rewindow", "-name="+n)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			go func() {
				if err := cmd.Run(); err != nil {
					log.Printf("%v: %v", cmd.Args, err)
				}
			}()
		}
	}
}

func (w *wsmgr) initWorkspaceLoaderTV() {
	tv, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal(err)
	}
	workspaceNameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal(err)
	}

	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			log.Fatal(err)
		}
		tvc.SetTitle("load workspace…")
		renderer := workspaceNameRenderer // for convenience
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 0 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	store, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_INT64)
	if err != nil {
		log.Fatal(err)
	}
	updateConfiguredWorkspaces(store)
	tv.SetModel(store)

	tv.Connect("row-activated", func(tv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		iter, err := store.GetIter(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}

		nameval, err := store.GetValue(iter, 0)
		if err != nil {
			log.Fatalf("BUG: GetValue(0) = %v", err)
		}
		name, err := nameval.GetString()
		if err != nil {
			log.Fatalf("BUG: GetString() = %v", err)
		}

		w.loadWorkspace(name)
	})

	w.workspaceLoaderTV = tv
}

func (w *wsmgr) updateWorkspaces() {
	w.currentWorkspace.ignoreEvents = true
	workspaces, err := i3.GetWorkspaces()
	if err != nil {
		log.Fatal(err)
	}
	store := w.currentWorkspace.store
	store.Clear()
	for _, ws := range workspaces {
		store.Set(store.Append(), []int{0, 1, 2}, []interface{}{ws.Num, ws.Name, ws.ID})
	}
	w.currentWorkspace.ignoreEvents = false
}

func (w *wsmgr) workspaceFromIter(iter *gtk.TreeIter) i3.Workspace {
	store := w.currentWorkspace.store

	numval, err := store.GetValue(iter, 0)
	if err != nil {
		log.Fatalf("BUG: GetValue(0) = %v", err)
	}
	num, err := numval.GoValue()
	if err != nil {
		log.Fatalf("BUG: GoValue() = %v", err)
	}

	nameval, err := store.GetValue(iter, 1)
	if err != nil {
		log.Fatalf("BUG: GetValue(0) = %v", err)
	}
	name, err := nameval.GetString()
	if err != nil {
		log.Fatalf("BUG: GetString() = %v", err)
	}

	idval, err := store.GetValue(iter, 2)
	if err != nil {
		log.Fatalf("BUG: GetValue(2) = %v", err)
	}
	id, err := idval.GoValue()
	if err != nil {
		log.Fatalf("BUG: GoValue() = %v", err)
	}
	return i3.Workspace{
		ID:   i3.WorkspaceID(id.(int64)),
		Num:  num.(int64),
		Name: name,
	}
}

func (w *wsmgr) workspaceFromPath(path string) i3.Workspace {
	iter, err := w.currentWorkspace.store.GetIterFromString(path)
	if err != nil {
		log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
	}
	return w.workspaceFromIter(iter)
}

func (w *wsmgr) initCurrentWorkspaceTV() {
	tv, err := gtk.TreeViewNew()
	if err != nil {
		log.Fatal(err)
	}
	workspaceNameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Fatal(err)
	}

	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			log.Fatal(err)
		}
		tvc.SetTitle("number")
		renderer, err := gtk.CellRendererTextNew()
		if err != nil {
			log.Fatal(err)
		}
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 0 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	var titleColumn *gtk.TreeViewColumn
	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			log.Fatal(err)
		}
		titleColumn = tvc
		tvc.SetTitle("name")
		renderer := workspaceNameRenderer // for convenience
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 1 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	// TODO: could we implement a custom model? https://github.com/gotk3/gotk3/issues/721
	// Maybe that would free us from doing the awkward putting/getting into a gtk.ListStore
	store, err := gtk.ListStoreNew(glib.TYPE_INT64, glib.TYPE_STRING, glib.TYPE_INT64)
	if err != nil {
		log.Fatal(err)
	}
	w.currentWorkspace.store = store
	w.updateWorkspaces()

	tv.SetModel(store)

	if iter, ok := store.GetIterFirst(); ok {
		focused := int64(1)
		workspaces, err := i3.GetWorkspaces()
		if err != nil {
			log.Fatal(err)
		}
		for _, ws := range workspaces {
			if ws.Focused {
				focused = ws.Num
				break
			}
		}
		for {
			ws := w.workspaceFromIter(iter)
			if ws.Num == focused {
				path, err := store.GetPath(iter)
				if err != nil {
					log.Printf("GetPath(%v): %v", iter, err)
					break
				}
				tv.SetCursor(path, titleColumn, false /* startEditing */)
				break
			}
			if !store.IterNext(iter) {
				break
			}
		}
	}

	workspaceNameRenderer.SetProperty("editable", true)
	workspaceNameRenderer.Connect("edited", func(cell *gtk.CellRendererText, path string, newText string) {
		iter, err := store.GetIterFromString(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}

		existing := w.workspaceFromPath(path)
		numPrefix := fmt.Sprintf("%d: ", existing.Num)
		if !strings.HasPrefix(newText, numPrefix) {
			newText = numPrefix + newText
		}

		cmd := fmt.Sprintf(`rename workspace "%s" to "%s"`, existing.Name, newText)
		log.Printf("renaming workspace: %q", cmd)
		if _, err := i3.RunCommand(cmd); err != nil {
			log.Print(err)
			return
		}
		store.Set(iter, []int{0, 1, 2}, []interface{}{
			existing.Num, newText, existing.ID,
		})
	})

	tv.SetReorderable(true)

	store.Connect("row-inserted", func(model gtk.ITreeModel, path *gtk.TreePath, iter *gtk.TreeIter) {
		if w.currentWorkspace.ignoreEvents {
			return // currently updating
		}
		// NOTE: the row was inserted without any data yet, the row-changed
		// signal will be emitted once the data is copied.
		log.Printf("row-inserted, path %v", path)

		// Keep the TreeView selection on the moved workspace
		tv.SetCursor(path, titleColumn, false)
	})
	//var changedWorkspace i3.Workspace
	store.Connect("row-changed", func(model gtk.ITreeModel, path *gtk.TreePath, iter *gtk.TreeIter) {
		if w.currentWorkspace.ignoreEvents {
			return // currently updating
		}

		log.Printf("row-changed, path %v", path)
		//changedWorkspace = workspaceFromPath(path.String())
	})
	store.Connect("row-deleted", func(model gtk.ITreeModel, path *gtk.TreePath) {
		if w.currentWorkspace.ignoreEvents {
			return // currently updating
		}

		log.Printf("row-deleted, path %v", path)

		var num int64
		for iter, ok := store.GetIterFirst(); ok; ok = store.IterNext(iter) {
			num++
			ws := w.workspaceFromIter(iter)
			log.Printf("  ws = %+v", ws)
			if ws.Num == num {
				continue // no rename required
			}
			oldName := ws.Name
			numPrefix := fmt.Sprintf("%d: ", ws.Num)
			if strings.Contains(ws.Name, ":") {
				// Named workspace
				ws.Name = fmt.Sprintf("%d: %s", num, strings.TrimPrefix(ws.Name, numPrefix))
			} else {
				// Numbered workspace
				ws.Name = fmt.Sprintf("%d", num)
			}
			rename := fmt.Sprintf(`rename workspace "%s" to "%s"`, oldName, ws.Name)
			log.Printf("  -> rename=%q", rename)
			if _, err := i3.RunCommand(rename); err != nil {
				log.Fatal(err)
			}
		}

		w.updateWorkspaces()
	})

	// When double-clicking a workspace, move our window to the workspace, then
	// switch to the workspace. This allows for quickly getting an overview of
	// which windows are present on which workspace, without having to deal with
	// moving windows around manually.
	tv.Connect("row-activated", func(tv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		activated := w.workspaceFromPath(path.String())
		log.Printf("row-activated signal for workspace %+v", activated)
		cmd := fmt.Sprintf(`move container to workspace "%s"; workspace "%s"`, activated.Name, activated.Name)
		if _, err := i3.RunCommand(cmd); err != nil {
			log.Fatal(err)
		}
	})

	w.currentWorkspace.tv = tv
}

func (w *wsmgr) addWorkspace(name string) {
	store := w.currentWorkspace.store
	var highest int64
	for iter, ok := store.GetIterFirst(); ok; ok = store.IterNext(iter) {
		ws := w.workspaceFromIter(iter)
		if ws.Num > highest {
			highest = ws.Num
		}
	}
	newName := fmt.Sprintf("%d: %s", highest+1, name)

	cmd := fmt.Sprintf(`move container to workspace "%s"; workspace "%s"`, newName, newName)
	if _, err := i3.RunCommand(cmd); err != nil {
		log.Fatal(err)
	}

	w.updateWorkspaces()
}

func (w *wsmgr) initAddWorkspaceButton() {
	addButton, err := gtk.ButtonNewWithMnemonic("_add workspace")
	if err != nil {
		log.Fatal(err)
	}
	addButton.Connect("clicked", func() {
		log.Printf("adding new workspace")
		w.addWorkspace("unnamed")
	})
	w.addWorkspaceButton = addButton
}

//go:embed "logo.png"
var logoPNG []byte

func setIconFromEmbeddedResource(b []byte, win *gtk.Window) error {
	f, err := ioutil.TempFile("", "logopng")
	if err != nil {
		return err
	}

	if _, err := f.Write(logoPNG); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if err := win.SetIconFromFile(f.Name()); err != nil {
		return err
	}

	if err := os.Remove(f.Name()); err != nil {
		return err
	}

	return nil
}

func autosave() error {
	workspaces, err := i3.GetWorkspaces()
	if err != nil {
		log.Fatal(err)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	autosaveFile := filepath.Join(configDir, "wsmgr-for-i3", "autosave.json")
	f, err := renameio.TempFile("", autosaveFile)
	if err != nil {
		return err
	}
	defer f.Cleanup()
	b, err := json.Marshal(workspaces)
	if err != nil {
		return err
	}
	f.Write(b)
	return f.CloseAtomicallyReplace()
}

var rootCmd = &cobra.Command{
	Use:   "wsmgr",
	Short: "workspace manager",
	Long:  "workspace manager",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize GTK without parsing any command line arguments.
		gtk.Init(nil)

		cssProvider, err := gtk.CssProviderNew()
		if err != nil {
			return err
		}

		// TODO: can we use box-shadow?
		// https://developer.gnome.org/gtk3/stable/chap-css-overview.html
		// https://developer.gnome.org/gtk3/stable/chap-css-properties.html
		cssProvider.LoadFromData(css)

		defaultScreen, err := gdk.ScreenGetDefault()
		if err != nil {
			return err
		}
		gtk.AddProviderForScreen(defaultScreen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

		// Create a new toplevel window, set its title, and connect it to the
		// "destroy" signal to exit the GTK main loop when it is destroyed.
		win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
		if err != nil {
			return err
		}
		win.SetName("win")
		win.SetModal(true)

		win.SetTitle("wsmgr-for-i3 workspace manager")
		win.Connect("destroy", func() {
			gtk.MainQuit()
		})

		if err := setIconFromEmbeddedResource(logoPNG, win); err != nil {
			log.Print(err)
		}

		w := &wsmgr{}
		w.initCurrentWorkspaceTV()
		w.initAddWorkspaceButton()
		w.initWorkspaceLoaderTV()

		vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
		if err != nil {
			return err
		}
		vbox.PackStart(w.currentWorkspace.tv, true, true, 5)
		vbox.PackStart(w.addWorkspaceButton, false, false, 5)
		vbox.PackStart(w.workspaceLoaderTV, false, false, 5)
		win.Add(vbox)

		// Set the default window size.
		win.SetDefaultSize(800, 600)

		// Recursively show all widgets contained in this window.
		win.ShowAll()

		// Begin executing the GTK main loop.  This blocks until
		// gtk.MainQuit() is run.
		gtk.Main()

		return nil
	},
}

var autosaveCmd = &cobra.Command{
	Use:   "autosave",
	Short: "save workspace names to the autosave file",
	Long:  "do not show the GUI, instead save workspace names to the autosave file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return autosave()
	},
}

func ws() error {
	rootCmd.AddCommand(autosaveCmd)

	if err := rootCmd.Execute(); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := ws(); err != nil {
		log.Fatal(err)
	}
}
