package main

import (
	"os"
	"strings"
)

type Environment struct {
	EmailPassword string
	SafePath      string
	SynergyPath   string
}

func envs() Environment {
	environment := Environment{}
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "FREEHANDLE_SECRET=") {
			environment.EmailPassword, _ = strings.CutPrefix(env, "FREEHANDLE_SECRET=")
		} else if strings.HasPrefix(env, "SAFE=") {
			environment.SafePath, _ = strings.CutPrefix(env, "SAFE=")
		} else if strings.HasPrefix(env, "SYNERGY_PATH=") {
			environment.SynergyPath, _ = strings.CutPrefix(env, "SYNERGY_PATH=")
		}
	}
	return environment
}
