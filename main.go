package main

import (
	"fmt"
	"github.com/golang/geo/r3"
	dem "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/common"
	events "github.com/markus-wa/demoinfocs-golang/v2/pkg/demoinfocs/events"
	"log"
	"os"
	"path/filepath"
)

func main() {
	err := filepath.Walk("demos/",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if filepath.Ext(path) != ".dem" {
				fmt.Println("Ignoring file: " + path)
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				panic(err)
			}

			fmt.Println(path, info.Size())

			defer f.Close()

			fmt.Printf("Analyzing file: %s\n", f.Name())
			output := parseFile(f)
			fmt.Printf("Finished file: %s\n\n", f.Name())

			writeToOutputFile(output)
			return nil
		})
	if err != nil {
		panic(err)
	}
}

func writeToOutputFile(output string) {
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile("output.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte(output)); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func parseFile(f *os.File) string {
	p := dem.NewParser(f)
	defer p.Close()

	output := ""
	var isInRound bool
	var matchEnded bool
	var spectators []*common.Player
	positions := make(map[string]map[string]*PositionInfo)

	output += "Result for file: " + f.Name() + "\n"

	// Register handler on kill events
	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		isInRound = true
		spectators = nil
		for _, participant := range p.GameState().Participants().Connected() {
			if participant.Team == 1 {
				spectators = append(spectators, participant)
			}
		}
	})

	//p.RegisterEventHandler(func(e events.GenericGameEvent) {
	//fmt.Println("Tick: " + strconv.Itoa(p.GameState().IngameTick()))

	//})

	p.RegisterEventHandler(func(e events.RoundEnd) {
		isInRound = false
	})

	for matchEnded == false {
		frame, err := p.ParseNextFrame()
		if !frame {
			matchEnded = true
			break
		}

		if err != nil {
			fmt.Println("Parser got error")
		}

		if isInRound && p.GameState().IngameTick() % 10 == 0 {
			analyzeCamPos(p.GameState(), &spectators, positions)
		}
	}

	for name, positionMap := range positions {
		for _, positionInfo := range positionMap {
			if positionInfo.times > 40 {
				output += fmt.Sprintf("%s has been in this pos %d times\n Info: %#v\n", name, positionInfo.times, positionInfo)
			}
		}
	}

	output += "\n\n"
	
	return output
}

func analyzeCamPos(gameState dem.GameState, spectators *[]*common.Player, positions map[string]map[string]*PositionInfo) {
	for _, spectator := range *spectators {
		player := gameState.Participants().ByUserID()[spectator.UserID]

		if player == nil {
			continue
		}

		spectatorPosition := player.Position()
		//fmt.Printf("%s : %s\n", spectator.Name, spectatorPosition)

		isPlayerPosition := false
		for _, player := range gameState.Participants().Playing() {
			if player.Position() == spectatorPosition {
				isPlayerPosition = true
				break
			}
		}

		if isPlayerPosition {
			continue;
		}

		if positions[spectator.Name] == nil {
			positions[spectator.Name] = make(map[string]*PositionInfo)
		}

		posKey := fmt.Sprintf("%s%d%d", spectatorPosition, gameState.TeamTerrorists().Score(), gameState.TeamCounterTerrorists().Score())
		info, ok := positions[spectator.Name][posKey]

		if !ok {
			positions[spectator.Name][posKey] = &PositionInfo{
				position: spectatorPosition,
				steamId: fmt.Sprintf("%d", spectator.SteamID64),
				times:     1,
				ctScore:   gameState.TeamCounterTerrorists().Score(),
				tScore:    gameState.TeamTerrorists().Score(),
				firstTick: gameState.IngameTick(),
				lastTick:  gameState.IngameTick(),
			}
		} else {
			info.times = info.times + 1
			info.lastTick = gameState.IngameTick()
		}
	}
}

type PositionInfo struct {
	position r3.Vector
	steamId string
	times int
	ctScore int
	tScore int
	firstTick int
	lastTick int
}

