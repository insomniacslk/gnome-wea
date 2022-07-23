package main

import "os"

var (
	DefaultEditorPath = os.ExpandEnv("$EDITOR")
	DefaultEditorArgs = []string{}
)
