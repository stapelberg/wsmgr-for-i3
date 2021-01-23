package main

import (
	"flag"
	"log"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"go.i3wm.org/i3/v4"
)

func ws() error {
	flag.Parse()
	log.Printf("hey")

	// Initialize GTK without parsing any command line arguments.
	gtk.Init(nil)

	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		return err
	}

	// TODO: can we use box-shadow?
	// https://developer.gnome.org/gtk3/stable/chap-css-overview.html
	// https://developer.gnome.org/gtk3/stable/chap-css-properties.html
	cssProvider.LoadFromData(`

#win,
#grid,
#icon,
#title,
#message,
#btnunlock {
  font-weight: bold;
  font-style: italic;
  color: #ffffff;
  background: #000000;
}

#grid {
  margin: 1rem;
  border: 1px solid grey;
  padding-left: 1rem;
  padding-right: 1rem;
}

#icon {
  margin-top: 1rem;
  /*background-color: yellow;*/
}

#title {
  padding: 1rem;
  font-size: 150%;
}

#message {
  padding: 1rem;
}

#btnunlock {
  margin: 1rem;
  padding: 1rem;
  border: 1px solid green;
}

#btnunlock:hover {
  background-color: #555;
}

#btnunlock:active {
  background-color: blue;
}
`)

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

	// TODO: add a treeview widget showing the current workspaces
	tv, err := gtk.TreeViewNew()
	if err != nil {
		return err
	}
	workspaceNameRenderer, err := gtk.CellRendererTextNew()
	if err != nil {
		return err
	}

	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			return err
		}
		tvc.SetTitle("number")
		renderer, err := gtk.CellRendererTextNew()
		if err != nil {
			return err
		}
		tvc.PackStart(renderer, true)
		tvc.AddAttribute(renderer, "text", 0 /* references column 0 in model */)
		tv.AppendColumn(tvc)
	}

	{
		tvc, err := gtk.TreeViewColumnNew()
		if err != nil {
			return err
		}
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
		return err
	}

	workspaces, err := i3.GetWorkspaces()
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		store.Set(store.Append(), []int{0, 1, 2}, []interface{}{ws.Num, ws.Name, ws.ID})

	}
	// TODO: flag for test dataset maybe?
	//store.Set(store.Append(), []int{0, 1}, []interface{}{23, "bar"})

	tv.SetModel(store)

	workspaceFromPath := func(path string) i3.Workspace {
		iter, err := store.GetIterFromString(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}
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

	workspaceNameRenderer.SetProperty("editable", true)
	workspaceNameRenderer.Connect("edited", func(cell *gtk.CellRendererText, path string, newText string) {
		log.Printf("edited!")
		log.Printf("path = %q", path)
		log.Printf("newText = %q", newText)

		iter, err := store.GetIterFromString(path)
		if err != nil {
			log.Fatalf("BUG: GetIterFromString(%q) = %v", path, err)
		}

		existing := workspaceFromPath(path)

		log.Printf("TODO: [workspace=%d] rename workspace to: ", existing.ID)
		store.Set(iter, []int{0, 1, 2}, []interface{}{
			existing.Num, newText, existing.ID,
		})
	})

	tv.SetReorderable(true)
	store.Connect("row-inserted", func(model gtk.ITreeModel, path *gtk.TreePath, iter *gtk.TreeIter) {
		// TODO: for some reason, store.GetValue(iter) returns a gchararray, but
		// GetString() returns null?!
	})
	store.Connect("row-deleted", func(model gtk.ITreeModel, path *gtk.TreePath) {
		log.Printf("row-deleted")
		// TODO: go through all entries and fix up the workspace numbers, doing
		// the i3 rename ipc commands as we go along
	})

	win.Add(tv)

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
