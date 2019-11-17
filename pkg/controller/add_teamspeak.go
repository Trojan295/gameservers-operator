package controller

import (
	"gitlab.com/chinchilla-games/gameservers-operator/pkg/controller/teamspeak"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, teamspeak.Add)
}
