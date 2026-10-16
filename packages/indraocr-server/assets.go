package indraocrserver

import _ "embed"

//go:embed requirements.txt
var Requirements string

//go:embed server.py
var Script []byte
