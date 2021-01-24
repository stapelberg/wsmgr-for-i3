package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"go.i3wm.org/i3/v4"
)

func updateWorkspaces(store *gtk.ListStore) error {
	workspaces, err := i3.GetWorkspaces()
	if err != nil {
		return err
	}
	store.Clear()
	for _, ws := range workspaces {
		store.Set(store.Append(), []int{0, 1, 2}, []interface{}{ws.Num, ws.Name, ws.ID})
	}
	return nil
}

func workspaceManagement() (*gtk.TreeView, *gtk.Button, error) {
	tv, err := gtk.TreeViewNew()
	if err != nil {
		return nil, nil, err
	}
	workspaceNameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return nil, nil, err
	}

	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			return nil, nil, err
		}
		tvc.SetTitle("number")
		renderer, err := gtk.CellRendererTextNew()
		if err != nil {
			return nil, nil, err
		}
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 0 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	var titleColumn *gtk.TreeViewColumn
	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			return nil, nil, err
		}
		titleColumn = tvc
		tvc.SetTitle("title")
		renderer := workspaceNameRenderer // for convenience
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 1 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	// TODO: could we implement a custom model? https://github.com/gotk3/gotk3/issues/721
	// Maybe that would free us from doing the awkward putting/getting into a gtk.ListStore
	store, err := gtk.ListStoreNew(glib.TYPE_INT64, glib.TYPE_STRING, glib.TYPE_INT64)
	if err != nil {
		return nil, nil, err
	}
	if err := updateWorkspaces(store); err != nil {
		return nil, nil, err
	}

	tv.SetModel(store)

	workspaceFromIter := func(iter *gtk.TreeIter) i3.Workspace {
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
	workspaceFromPath := func(path string) i3.Workspace {
		iter, err := store.GetIterFromString(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}
		return workspaceFromIter(iter)
	}

	workspaceNameRenderer.SetProperty("editable", true)
	workspaceNameRenderer.Connect("edited", func(cell *gtk.CellRendererText, path string, newText string) {
		iter, err := store.GetIterFromString(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}

		existing := workspaceFromPath(path)
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

	ignoreEvents := false
	store.Connect("row-inserted", func(model gtk.ITreeModel, path *gtk.TreePath, iter *gtk.TreeIter) {
		if ignoreEvents {
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
		if ignoreEvents {
			return // currently updating
		}

		log.Printf("row-changed, path %v", path)
		//changedWorkspace = workspaceFromPath(path.String())
	})
	store.Connect("row-deleted", func(model gtk.ITreeModel, path *gtk.TreePath) {
		if ignoreEvents {
			return // currently updating
		}

		log.Printf("row-deleted, path %v", path)

		var num int64
		for iter, ok := store.GetIterFirst(); ok; ok = store.IterNext(iter) {
			num++
			ws := workspaceFromIter(iter)
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

		ignoreEvents = true
		if err := updateWorkspaces(store); err != nil {
			log.Print(err)
		}
		ignoreEvents = false
	})

	// When double-clicking a workspace, move our window to the workspace, then
	// switch to the workspace. This allows for quickly getting an overview of
	// which windows are present on which workspace, without having to deal with
	// moving windows around manually.
	tv.Connect("row-activated", func(tv *gtk.TreeView, path *gtk.TreePath, column *gtk.TreeViewColumn) {
		activated := workspaceFromPath(path.String())
		log.Printf("row-activated signal for workspace %+v", activated)
		cmd := fmt.Sprintf(`move container to workspace "%s"; workspace "%s"`, activated.Name, activated.Name)
		if _, err := i3.RunCommand(cmd); err != nil {
			log.Fatal(err)
		}
	})

	addButton, err := gtk.ButtonNewWithMnemonic("_add workspace")
	if err != nil {
		return nil, nil, err
	}
	addButton.Connect("clicked", func() {
		log.Printf("adding new workspace")

		var highest int64
		for iter, ok := store.GetIterFirst(); ok; ok = store.IterNext(iter) {
			ws := workspaceFromIter(iter)
			if ws.Num > highest {
				highest = ws.Num
			}
		}
		newName := fmt.Sprintf("%d: unnamed", highest+1)
		cmd := fmt.Sprintf(`move container to workspace "%s"; workspace "%s"`, newName, newName)
		if _, err := i3.RunCommand(cmd); err != nil {
			log.Fatal(err)
		}

		ignoreEvents = true
		if err := updateWorkspaces(store); err != nil {
			log.Print(err)
		}
		ignoreEvents = false
	})
	return tv, addButton, nil
}

func ws() error {
	flag.Parse()

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

	win.SetTitle("i3 workspaces")
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	vbox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		return err
	}
	tv, addButton, err := workspaceManagement()
	if err != nil {
		return err
	}
	vbox.PackStart(tv, true, true, 5)
	vbox.PackStart(addButton, false, false, 5)
	win.Add(vbox)

	// Set the default window size.
	win.SetDefaultSize(800, 600)

	// Recursively show all widgets contained in this window.
	win.ShowAll()

	// Begin executing the GTK main loop.  This blocks until
	// gtk.MainQuit() is run.
	gtk.Main()

	return nil
}

func main() {
	if err := ws(); err != nil {
		log.Fatal(err)
	}
}
