// Copyright 2016 Bryan Jeal <bryan@jeal.ca>

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tmpl

import (
	"errors"
	"html/template"
	"path/filepath"
	"strings"
	"sync"

	helpers "github.com/bryanjeal/go-helpers"
	"github.com/fsnotify/fsnotify"
	memdb "github.com/hashicorp/go-memdb"
)

// Errors
var (
	ErrNoName       = errors.New("no template name supplied")
	ErrTmplNotFound = errors.New("template not found in store")
	ErrTmplExists   = errors.New("template with existing name found in store")
	ErrNoTmpl       = errors.New("no template data provided")
)

// Ctx is a common context for other modules to embed/use
type Ctx struct {
	Flashes      []interface{}
	FlashesInfo  []interface{}
	FlashesWarn  []interface{}
	FlashesError []interface{}
	CsrfToken    string
	Data         map[string]interface{}
}

// EmailMessage represents the reusable core of an email
type EmailMessage struct {
	From      string
	Subject   string
	PlainText string
	TplName   string
}

// TplSys is the template helper system
type TplSys struct {
	baseDir string
	funcMap template.FuncMap
	store   *tmplStore
}

// tmplStore has a mutex to control access to it
// tmpls is a map with key: <template name>; value: *template.Template
type tmplStore struct {
	*sync.RWMutex
	tmpls         map[string]*template.Template
	tmplDB        *memdb.MemDB
	tmplWatch     *fsnotify.Watcher
	tmplWatchQuit chan bool
}

// NewTplSys created a new template helper system
func NewTplSys(basedir string) *TplSys {
	t := &TplSys{
		baseDir: basedir,
		store: &tmplStore{
			RWMutex:       &sync.RWMutex{},
			tmpls:         make(map[string]*template.Template),
			tmplDB:        memdbMust(memdb.NewMemDB(schema)),
			tmplWatch:     fsnotifyMust(fsnotify.NewWatcher()),
			tmplWatchQuit: make(chan bool),
		},
	}
	t.funcMap = t.genFuncMap()
	go t.handleWatcherEvents()
	return t
}

// BaseDir returns the template base directory
func (t *TplSys) BaseDir() string {
	return t.baseDir
}

// InitializeStore resets template store and file watcher
// If you change Tpl.BaseDir then you MUST run InitializeStore()
func (t *TplSys) InitializeStore() {
	t.store.Lock()
	t.store.tmpls = make(map[string]*template.Template)
	t.store.tmplDB = memdbMust(memdb.NewMemDB(schema))
	t.store.tmplWatch.Close()
	t.store.tmplWatchQuit <- true
	t.store.tmplWatch = fsnotifyMust(fsnotify.NewWatcher())
	t.store.Unlock()

	go t.handleWatcherEvents()
}

// AddTemplate will add a *template.Template to Tpl.store with "name".
// If baseTmpl is not empty then find baseTmpl in store and clone it. Proceed as usual.
// If store already has a template with "name" then an error will be returned
func (t *TplSys) AddTemplate(name, baseTmpl, tmplSrc string, filenames ...string) (*template.Template, error) {
	// make sure a template with the same name doesn't exist
	_, err := t.getTemplate(name)
	if err == nil {
		return nil, ErrTmplExists
	} else if err != ErrTmplNotFound {
		return nil, err
	}

	tmpl, err := t.saveTemplate(name, baseTmpl, true, tmplSrc, filenames...)
	return tmpl, err
}

// PutTemplate will put a *template.Template to Tpl.store with "name".
// Unlike AddTemplate this will override existing templates
func (t *TplSys) PutTemplate(name, baseTmpl, tmplSrc string, filenames ...string) (*template.Template, error) {
	// try and get template. If one exists then isNew is false
	isNew := false
	_, err := t.getTemplate(name)
	if err == ErrTmplNotFound {
		isNew = true
	} else if err != nil {
		return nil, err
	}

	tmpl, err := t.saveTemplate(name, baseTmpl, isNew, tmplSrc, filenames...)
	return tmpl, err
}

// ExecuteTemplate will find the template with "name" and execute it with the provided context
// If template with "name" doesn't exist then an error will be returned
func (t *TplSys) ExecuteTemplate(name string, ctx interface{}) ([]byte, error) {
	tmpl, err := t.getTemplate(name)
	if err != nil {
		return nil, err
	}

	b := helpers.BufferPool.Get()
	defer helpers.BufferPool.Put(b)

	// execute template
	err = template.Must(tmpl.Clone()).Execute(b, ctx)
	if err != nil {
		return nil, err
	}

	// return bytes
	return b.Bytes(), nil
}

func (t *TplSys) getTemplate(name string) (*template.Template, error) {
	err := t.checkName(name)
	if err != nil {
		return nil, err
	}

	t.store.RLock()
	defer t.store.RUnlock()
	tmpl, ok := t.store.tmpls[name]
	if !ok {
		return nil, ErrTmplNotFound
	}

	return tmpl, nil
}

func (t *TplSys) saveTemplate(name, baseTmpl string, isNew bool, tmplSrc string, filenames ...string) (*template.Template, error) {
	err := t.checkName(name)
	if err != nil {
		return nil, err
	}

	// check baseTmpl name
	// if one isn't passed then create a new template
	// otherwise get baseTmpl and clone it
	var tmpl *template.Template
	hasBaseTmpl := false
	err = t.checkName(baseTmpl)
	if err == ErrNoName {
		tmpl = template.New(name).Funcs(t.funcMap)
	} else {
		hasBaseTmpl = true
		tmpl, err = t.getTemplate(baseTmpl)
		if err != nil {
			return nil, err
		}
		tmpl, err = tmpl.Clone()
		if err != nil {
			return nil, err
		}
	}

	// make sure template data or file(s) are passed
	hasSrc := false
	tmplSrc = strings.TrimSpace(tmplSrc)
	if len(tmplSrc) == 0 {
		if len(filenames) == 0 {
			return nil, ErrNoTmpl
		}
		// clean filenames
		for i, f := range filenames {
			f = strings.TrimPrefix(f, t.BaseDir())
			f = strings.TrimPrefix(f, "/")
			f = filepath.Join(t.BaseDir(), f)
			filenames[i] = f
		}
		// parse the files
		tmpl, err = tmpl.ParseFiles(filenames...)
	} else {
		hasSrc = true
		tmpl, err = tmpl.Parse(tmplSrc)
	}
	if err != nil {
		return nil, err
	}

	// add template to template store
	t.store.Lock()
	defer t.store.Unlock()

	// store template
	t.store.tmpls[name] = tmpl

	// push template data to tmplDB
	err = t.saveTemplateDataToDB(&tmplData{
		Name:        name,
		BaseTmplID:  baseTmpl,
		Src:         tmplSrc,
		Filenames:   filenames,
		HasSrc:      hasSrc,
		HasBaseTmpl: hasBaseTmpl,
	})
	if err != nil {
		return nil, err
	}

	// if template isn't new then rebuild all children templates
	if isNew == false {
		err = t.rebuildChildTemplates(name, tmpl)
		if err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}

func (t *TplSys) checkName(name string) error {
	if len(strings.TrimSpace(name)) == 0 {
		return ErrNoName
	}
	return nil
}

// rebuildChildTemplates will be called recursively, so we don't lock the store
// the initial call to this function should take care of locking
func (t *TplSys) rebuildChildTemplates(name string, tmpl *template.Template) error {
	// make sure template name is passed
	err := t.checkName(name)
	if err != nil {
		return err
	}

	// get all subtemplates that are based on this one
	tx := t.store.tmplDB.Txn(false)
	result, err := tx.Get("tmplData", "baseid", name)
	if err != nil {
		return err
	}

	// iterate over child templates and rebuild
	for r := result.Next(); r != nil; r = result.Next() {
		td := r.(*tmplData)
		ctmpl, err := tmpl.Clone()
		if err != nil {
			return err
		}

		// make sure template data or file(s) are passed
		if td.HasSrc {
			ctmpl, err = ctmpl.Parse(td.Src)
		} else {
			ctmpl, err = ctmpl.ParseFiles(td.Filenames...)
		}
		if err != nil {
			return err
		}

		// put template in store
		t.store.tmpls[td.Name] = ctmpl

		// rebuild all child templates
		err = t.rebuildChildTemplates(td.Name, ctmpl)
		if err != nil {
			return err
		}
	}

	// noop for "Read" transaction but included so I don't go WTF later.
	tx.Commit()

	return nil
}
