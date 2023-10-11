module github.com/freehandle/cb

replace github.com/freehandle/breeze => ../breeze

replace github.com/freehandle/axe => ../axe

replace github.com/freehandle/papirus => ../papirus

go 1.20

require (
	github.com/freehandle/axe v0.0.0-00010101000000-000000000000
	github.com/freehandle/breeze v0.0.0-00010101000000-000000000000
	github.com/freehandle/papirus v0.0.0-00010101000000-000000000000
	golang.org/x/term v0.12.0
)

require golang.org/x/sys v0.12.0 // indirect
