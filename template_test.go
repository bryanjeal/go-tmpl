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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type templateData struct {
	Config interface{}
	Data   interface{}
}

var tTmplData = templateData{
	Config: struct {
		Title       string
		Description string
		StaticURL   string
	}{
		Title:       "Test Title",
		Description: "Test Description",
		StaticURL:   "https://example.com/static/",
	},
	Data: struct {
		User    interface{}
		CSRF    string
		Flashes interface{}
	}{
		User:    nil,
		CSRF:    "csrf_token",
		Flashes: nil,
	},
}

func TestTemplate(t *testing.T) {
	dir, err := ioutil.TempDir(".", "testData-")
	if err != nil {
		t.Fatalf("Expected to make a temporary directory. Instead got the error: %v", err)
	}

	Tpl := NewTplSys(dir + "/")

	// make dirs for test templates
	err = os.MkdirAll(filepath.Join(dir, "layout"), 0777)
	if err != nil {
		t.Fatalf("Expected to make a layout directory. Instead got the error: %v", err)
	}
	err = os.MkdirAll(filepath.Join(dir, "content"), 0777)
	if err != nil {
		t.Fatalf("Expected to make a content directory. Instead got the error: %v", err)
	}
	err = os.MkdirAll(filepath.Join(dir, "partials"), 0777)
	if err != nil {
		t.Fatalf("Expected to make a partials directory. Instead got the error: %v", err)
	}
	err = os.MkdirAll(filepath.Join(dir, "output"), 0777)
	if err != nil {
		t.Fatalf("Expected to make a output directory. Instead got the error: %v", err)
	}

	// write test templates
	err = ioutil.WriteFile(filepath.Join(dir, "layout", "_base.html"), []byte(baseHTML), 0644)
	if err != nil {
		t.Fatalf("Expected to write test template. Instead got the error: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "content", "index.html"), []byte(indexHTML), 0644)
	if err != nil {
		t.Fatalf("Expected to write test template. Instead got the error: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "content", "login.html"), []byte(loginHTML), 0644)
	if err != nil {
		t.Fatalf("Expected to write test template. Instead got the error: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "partials", "_footer.html"), []byte(footerHTML), 0644)
	if err != nil {
		t.Fatalf("Expected to write test template. Instead got the error: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "partials", "_header.html"), []byte(headerHTML), 0644)
	if err != nil {
		t.Fatalf("Expected to write test template. Instead got the error: %v", err)
	}

	// Run tests
	t.Run("AddTemplate", func(t *testing.T) {
		tmpl, err := Tpl.AddTemplate("_base.html", "", "", "layout/_base.html")
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		if tmpl == nil {
			t.Fatalf("Expected to add template to store. Instead got nil template.")
		}

		_, err = Tpl.getTemplate("_base.html")
		if err != nil {
			t.Fatalf("Expected to get \"base\" template from store. Instead got the error: %v", err)
		}

		tmpl, err = Tpl.AddTemplate("_base-inline.html", "", baseHTML)
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		if tmpl == nil {
			t.Fatalf("Expected to add template to store. Instead got nil template.")
		}

		_, err = Tpl.getTemplate("_base-inline.html")
		if err != nil {
			t.Fatalf("Expected to get \"base\" template from store. Instead got the error: %v", err)
		}

		_, err = Tpl.AddTemplate("", "", "", "", "layout/_base.html")
		if err != ErrNoName {
			t.Fatalf("Expected ErrNoName. Instead got: %v", err)
		}

		_, err = Tpl.AddTemplate("index.html", "_base.html", "", "content/index.html")
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}

		_, err = Tpl.AddTemplate("login.html", "_base.html", loginHTML)
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}

		// cleanup
		Tpl.InitializeStore()
	})

	t.Run("AddTemplateDuplicate", func(t *testing.T) {
		_, err := Tpl.AddTemplate("_base.html", "", baseHTML)
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		_, err = Tpl.AddTemplate("_base.html", "", "", "layout/_base.html")
		if err != ErrTmplExists {
			t.Fatalf("Expected ErrTmplExists. Instead got: %v", err)
		}

		// cleanup
		Tpl.InitializeStore()
	})

	t.Run("PutTemplate", func(t *testing.T) {
		tmpl, err := Tpl.AddTemplate("_base.html", "", "", "layout/_base.html")
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		if tmpl == nil {
			t.Fatalf("Expected to add template to store. Instead got nil template.")
		}

		tmpl, err = Tpl.PutTemplate("_base.html", "", baseHTML)
		if err != nil {
			t.Fatalf("Expected to put template to store (overriding existing \"base\"). Instead got the error: %v", err)
		}
		if tmpl == nil {
			t.Fatalf("Expected to put template to store (overriding existing \"base\"). Instead got nil template.")
		}

		// cleanup
		Tpl.InitializeStore()
	})

	t.Run("ExecuteTemplate", func(t *testing.T) {
		// base template
		_, err := Tpl.AddTemplate("_base.html", "", "", "layout/_base.html")
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		d, err := Tpl.ExecuteTemplate("_base.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-base.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// index
		_, err = Tpl.AddTemplate("index.html", "_base.html", "", "content/index.html")
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		d, err = Tpl.ExecuteTemplate("index.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-index.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// login
		_, err = Tpl.AddTemplate("login.html", "_base.html", loginHTML)
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		d, err = Tpl.ExecuteTemplate("login.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-login.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// change base template by updating the html file
		err = ioutil.WriteFile(filepath.Join(Tpl.BaseDir(), "layout", "_base.html"), []byte(strings.Replace(baseHTML, "{{ partial \"_footer.html\" . }}", "", -1)), 0644)
		if err != nil {
			t.Fatalf("Expected to write test template. Instead got the error: %v", err)
		}

		// index
		time.Sleep(1 * time.Second)
		d, err = Tpl.ExecuteTemplate("index.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-index-base-changed.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// change base template with a tmplSrc
		_, err = Tpl.PutTemplate("_base.html", "", strings.Replace(baseHTML, "{{ partial \"_header.html\" . }}", "", -1))
		if err != nil {
			t.Fatalf("Expected to add template to store. Instead got the error: %v", err)
		}
		d, err = Tpl.ExecuteTemplate("_base.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-base-changed.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// make sure child page has changed
		d, err = Tpl.ExecuteTemplate("login.html", tTmplData)
		if err != nil {
			t.Fatalf("Expected to execute the template. Instead got the error: %v", err)
		}
		err = ioutil.WriteFile(Tpl.BaseDir()+"output/tmp-login-base-changed.html", d, 0644)
		if err != nil {
			t.Fatalf("Expected to write executed template to disk. Instead got the error: %v", err)
		}

		// cleanup
		Tpl.InitializeStore()
	})

	// clean up data
	err = os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Expected to remove test directories and files. Instead got the error: %v", err)
	}
}

// Test Data
var baseHTML = `
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <title>{{ .Config.Title }}</title>
        <meta name="description" content=" {{ .Config.Description }}">
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <link href="{{ .Config.StaticURL }}css/test.css" rel="stylesheet">
        {{ block "extra_css" . }} {{ end }}
    </head>
    <body>
        {{ partial "_header.html" . }}
        {{ block "content" . }} {{ end }}
        {{ partial "_footer.html" . }}
        <script src="{{ .Config.StaticURL }}js/test.js"></script>
        {{ block "extra_js" . }} {{ end }}
    </body>
</html>
`

var indexHTML = `
{{define "content"}}
<div class="container-fluid">
    <div class="row">
        <div class="col-sm-12">
            <h1>Test Site!</h1>
            <p>Welcome to the Test Index page! Enjoy your stay.</p>
            <p>Curabitur blandit tempus porttitor. Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus.</p>
            <p>Vivamus sagittis lacus vel augue laoreet rutrum faucibus dolor auctor. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Maecenas faucibus mollis interdum. Cras mattis consectetur purus sit amet fermentum. Fusce dapibus, tellus ac cursus commodo, tortor mauris condimentum nibh, ut fermentum massa justo sit amet risus. Praesent commodo cursus magna, vel scelerisque nisl consectetur et.</p>
        </div>
    </div>
</div>
{{ end }}
`

var loginHTML = `
{{define "extra_js"}}
<script src="{{ .Config.StaticURL }}js/login.js"></script>
{{end}}
{{define "content"}}
<div class="container">
    <div class="row">
        <div id="main-content" class="content-login col-sm-12">
            <!-- start: Content -->
            <!-- start: Login -->
            <div class="row">
                <div class="col-sm-8 col-sm-offset-2">
                    <div id="login-box" class="box padT50 padB100">
                        <div class="box-header">
                            <h2><i class="fa fa-user"></i><span class="break"></span>Login to your account</h2>
                            <div class="box-icon">
                                <a href="/" class="icon"><i class="fa fa-home"></i></a>
                            </div>
                        </div>
                        <div class="box-content">
                            <form action="/login" method="post">
                                <input type="hidden" name="csrf_token" value="{{ .Data.CSRF }}">
                                {{if .Data.Flashes }}
                                <div id="flashes">
                                    {{range .Data.Flashes }}
                                    <p class="alert alert-danger"> {{ . }} </p>
                                    {{ end }}
                                </div>
                                {{ end }}
                                <div class="login-fields row marT20">
                                    <!-- Text input-->
                                    <div class="form-group">
                                        <label class="col-md-3 control-label col-md-offset-1" for="email">Email</label>
                                        <div class="col-md-7">
                                            <input id="email" name="email" placeholder="your.email@example.com" class="form-control input-md" required="" type="text">
                                            <span class="help-block">Please type the Email you used to sign up.</span>
                                        </div>
                                    </div>

                                    <!-- Password input-->
                                    <div class="form-group">
                                        <label class="col-md-3 control-label col-md-offset-1" for="password">Password</label>
                                        <div class="col-md-7">
                                            <input id="password" name="password" placeholder="type your password" class="form-control input-md" required="" type="password">
                                            <span class="help-block">Please type the Password you used to sign up.</span>
                                        </div>
                                    </div>

                                    <!-- Button -->
                                    <div class="form-group">
                                        <label class="col-md-3 control-label col-md-offset-1" for="submit"></label>
                                        <div class="col-md-7">
                                            <button id="submit" name="submit" class="btn btn-primary btn-lg pull-right marT20">Login</button>
                                        </div>
                                    </div>
                                </div> <!-- /login-fields -->
                            </form>
                        </div><!--/box-content-->
                    </div><!--/box-->
                </div><!--/col-->
            </div>
            <!-- end: Login -->
            <!-- end: Content -->
        </div><!--/#content-->
    </div><!--/row-->
</div><!--/container-->
{{ end }}
`

var footerHTML = `
<footer>
    <div class="container">
        <div class="row">
            <div class="col-sm-12">
                <p class="text-center">&copy; Test Site Corp.</p>
            </div>
        </div><!--/row-->
    </div><!--/container-->
</footer>
`

var headerHTML = `
<!-- start: Header -->
<nav class="navbar navbar-default navbar-fixed-top" role="navigation">
<div class="container">
    <!-- Brand and toggle get grouped for better mobile display -->
    <div class="navbar-header">
        <button type="button" class="navbar-toggle" data-toggle="collapse" data-target="#primary-navbar-collapse">
            <span class="sr-only">Toggle navigation</span>
            <span class="icon-bar"></span>
            <span class="icon-bar"></span>
            <span class="icon-bar"></span>
        </button>
        <a class="navbar-brand" href="/">
            Test Site
        </a>
    </div>

    <div id="primary-navbar-collapse" class="collapse navbar-collapse">
        <ul class="nav navbar-nav navbar-right">
            {{if not .Data.User }}
            <li><a href="/login">Login</a></li>
            <li><a href="/coming-soon">Register</a></li>
            {{ else }}
            <li class="dropdown">
            <a href="#" class="dropdown-toggle" data-toggle="dropdown"><i class="fa  fa-user"></i> {{ .Data.User.FirstName }} <b class="caret"></b></a>
            <ul class="dropdown-menu">
                <li><a href="/settings"><i class="fa  fa-cog"></i> Settings</a></li>
                <li class="divider"></li>
                <li><a href="/logout"><i class="fa  fa-sign-out"></i>Logout</a></li>
            </ul>
            </li>
            {{ end }}
        </ul>
    </div><!-- /.navbar-collapse -->
</div><!-- /.container-->
</nav>
<!-- end: Header -->
`
