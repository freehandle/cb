module github.com/freehandle/cb

replace github.com/freehandle/breeze => ../breeze

replace github.com/freehandle/axe => ../axe

replace github.com/freehandle/papirus => ../papirus

replace github.com/freehandle/safe => ../safe

replace github.com/freehandle/synergy => ../synergy

go 1.20

require (
	github.com/freehandle/axe v0.0.0-00010101000000-000000000000
	github.com/freehandle/papirus v0.0.0-00010101000000-000000000000
	golang.org/x/term v0.13.0
)

require (
	github.com/freehandle/breeze v0.0.0-00010101000000-000000000000
	github.com/freehandle/safe v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.14.0
)

require (
	github.com/freehandle/synergy v0.0.0-00010101000000-000000000000 // indirect
	github.com/gomarkdown/markdown v0.0.0-20230922112808-5421fefb8386 // indirect
	golang.org/x/sys v0.13.0 // indirect
)
