package main

import "html/template"

const pageStyle = `<style>
body{font-family:system-ui,sans-serif;max-width:28rem;margin:3rem auto;padding:0 1rem;color:#1f2328}
h1{font-size:1.25rem}label{display:block;margin:.75rem 0 .25rem;font-weight:600}
input{width:100%;padding:.5rem;box-sizing:border-box;font-size:1rem}
.actions{display:flex;gap:.5rem;margin-top:1.25rem}button{padding:.5rem 1rem;font-size:1rem}
.note{color:#59636e;font-size:.875rem}code{background:#f6f8fa;padding:0 .25rem}
</style>`

var authorizeTmpl = template.Must(template.New("authorize").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Fake {{.Provider}} sign-in</title>` + pageStyle + `</head><body>
<h1>Fake {{.Provider}} sign-in</h1>
<p class="note">CourtKnights local OAuth fake. Any login is accepted. The same login always maps to the same provider user.</p>
<form method="post">
<input type="hidden" name="client_id" value="{{.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="hidden" name="scope" value="{{.Scope}}">
<label for="login">Login</label><input id="login" name="login" required autofocus placeholder="alice">
<label for="email">Email <span class="note">(default: login@example.com)</span></label><input id="email" name="email" type="email">
<label for="name">Name <span class="note">(default: login)</span></label><input id="name" name="name">
<div class="actions"><button name="action" value="approve">Sign in</button><button name="action" value="deny" formnovalidate>Deny</button></div>
</form>
<p class="note">Requested scope: <code>{{.Scope}}</code></p>
</body></html>`))

var deviceTmpl = template.Must(template.New("device").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Fake {{.Provider}} device sign-in</title>` + pageStyle + `</head><body>
<h1>Fake {{.Provider}} device sign-in</h1>
<p class="note">Enter the code shown by the CLI.</p>
<form method="post">
<label for="user_code">Code</label><input id="user_code" name="user_code" required value="{{.UserCode}}" placeholder="BCDF-GHJK">
<label for="login">Login</label><input id="login" name="login" placeholder="alice">
<label for="email">Email <span class="note">(default: login@example.com)</span></label><input id="email" name="email" type="email">
<label for="name">Name <span class="note">(default: login)</span></label><input id="name" name="name">
<div class="actions"><button name="action" value="approve">Approve</button><button name="action" value="deny">Deny</button></div>
</form>
</body></html>`))

var deviceResultTmpl = template.Must(template.New("device-result").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Fake {{.Provider}} device sign-in</title>` + pageStyle + `</head><body>
<h1>Fake {{.Provider}} device sign-in</h1>
<p>{{.Message}}</p>
</body></html>`))
