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
	"html/template"
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-memdb"
	uuid "github.com/satori/go.uuid"
)

type tmplData struct {
	Name        string
	BaseTmplID  string
	Src         string
	Filenames   []string
	HasSrc      bool
	HasBaseTmpl bool
}

type tmplFilename struct {
	ID       string
	Name     string
	Filename string
}

// Template Data DB schema
var schema = &memdb.DBSchema{
	Tables: map[string]*memdb.TableSchema{
		"tmplData": {
			Name: "tmplData",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.StringFieldIndex{Field: "Name"},
				},
				"baseid": {
					Name:         "baseid",
					AllowMissing: true,
					Indexer:      &memdb.StringFieldIndex{Field: "BaseTmplID"},
				},
			},
		},
		"tmplFilename": {
			Name: "tmplFilename",
			Indexes: map[string]*memdb.IndexSchema{
				"id": {
					Name:    "id",
					Unique:  true,
					Indexer: &memdb.UUIDFieldIndex{Field: "ID"},
				},
				"name": {
					Name:    "name",
					Indexer: &memdb.StringFieldIndex{Field: "Name"},
				},
				"filename": {
					Name:    "filename",
					Indexer: &memdb.StringFieldIndex{Field: "Filename"},
				},
			},
		},
	},
}

// memdbMust is a wrapper for memdb.NewMemDB() to ensure that only a *memdb.MemDB is passed through
func memdbMust(db *memdb.MemDB, err error) *memdb.MemDB {
	if err != nil {
		panic(err)
	}
	return db
}

func fsnotifyMust(w *fsnotify.Watcher, err error) *fsnotify.Watcher {
	if err != nil {
		panic(err)
	}
	return w
}

func (t *TplSys) saveTemplateDataToDB(td *tmplData) error {
	// Read from TmplDB and remove all old filepaths from watcher
	tx := t.store.tmplDB.Txn(false)
	result, err := tx.Get("tmplFilename", "name", td.Name)
	if err != nil {
		return err
	}

	// iterate over old filepaths and remove them
	for r := result.Next(); r != nil; r = result.Next() {
		tf := r.(*tmplFilename)
		err := t.store.tmplWatch.Remove(tf.Filename)
		if err != nil {
			return err
		}
	}

	// noop for "Read" transaction but included so I don't go WTF later.
	tx.Commit()

	// add or update tmplData and tmplFilename
	// Create a write transaction
	tx = t.store.tmplDB.Txn(true)

	// Insert or Update new tmplData
	if err := tx.Insert("tmplData", td); err != nil {
		tx.Abort()
		return err
	}

	// remove all old tmplFilename data
	if _, err := tx.DeleteAll("tmplFilename", "name", td.Name); err != nil {
		tx.Abort()
		return err
	}

	// if td.HasSrc is false then we have a list of filenames that we need to add
	if td.HasSrc == false {
		for _, f := range td.Filenames {
			tf := &tmplFilename{
				ID:       uuid.NewV4().String(),
				Name:     td.Name,
				Filename: f,
			}
			if err := tx.Insert("tmplFilename", tf); err != nil {
				tx.Abort()
				return err
			}
			err = t.store.tmplWatch.Add(f)
			if err != nil {
				return err
			}
		}
	}

	// Commit the transaction
	tx.Commit()
	return nil
}

func (t *TplSys) handleWatcherEvents() {
	for {
		select {
		case ev := <-t.store.tmplWatch.Events:
			if ev.Op&fsnotify.Write == fsnotify.Write {
				// Get template data from DB
				// Read from TmplDB and remove all old filepaths from watcher
				tx := t.store.tmplDB.Txn(false)
				result, err := tx.Get("tmplFilename", "filename", ev.Name)
				if err != nil {
					log.Println("error:", err)
				}

				// iterate over old filepaths and remove them
				for r := result.Next(); r != nil; r = result.Next() {
					tf := r.(*tmplFilename)

					tx2 := t.store.tmplDB.Txn(false)
					tdr, err := tx2.First("tmplData", "id", tf.Name)
					if err != nil {
						log.Println("error:", err)
					}
					td := tdr.(*tmplData)

					// get base template
					// or create new one
					tmpl := template.New(td.Name).Funcs(t.funcMap)
					if td.HasBaseTmpl {
						tmpl, err = t.getTemplate(td.BaseTmplID)
						tmpl, err = tmpl.Clone()
						if err != nil {
							log.Println("error:", err)
						}
					}
					tmpl, err = tmpl.ParseFiles(td.Filenames...)
					if err != nil {
						log.Println("error:", err)
					}

					t.store.Lock()
					// store updated template
					t.store.tmpls[td.Name] = tmpl

					// rebuild child templates
					err = t.rebuildChildTemplates(td.Name, tmpl)
					if err != nil {
						log.Println("error:", err)
					}
					t.store.Unlock()

					// noop for "Read" transaction but included so I don't go WTF later.
					tx2.Commit()
				}

				// noop for "Read" transaction but included so I don't go WTF later.
				tx.Commit()
				// Rebuild template
				log.Println("modified file:", ev.Name)
			}
		case err := <-t.store.tmplWatch.Errors:
			if err != nil {
				log.Println("error:", err)
			}
		case <-t.store.tmplWatchQuit:
			return
		}
	}
}
