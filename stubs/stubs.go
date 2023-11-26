package stubs

import "uk.ac.bris.cs/gameoflife/util"

var GameOfLifeUpdate = "GameOfLifeOperations.Update"
var Pause = "Operations.Pause"
var Quit = "Operations.Quit"
var SuperQuit = "Operations.SuperQuit"
var BrokeOps = "Operations.Run"
var Retrieve = "Operations.RetrieveCurrentData"
var WorkerQuit = "GameOfLifeOperations.WorkerQuit"

type Parameters struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type Request struct {
	World       [][]byte
	Turns       int
	ImageHeight int
	ImageWidth  int
	Threads     int
	EndY        int
	StartY      int
	Worker      int
}

type Response struct {
	Alive          []util.Cell
	AliveCount     int
	TurnsCompleted int
	World          [][]byte
	WorkSlice      [][]byte
	Worker         int
}
