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

	// StaticPositions by Name and PositionKey (Coordinates CTscore Tscore)
	positionsByNameAndPosKey := make(map[string]map[string]*StaticPositionInfo)

	// FollowPositionList by Name
	positionsListByName := make(map[string][]FollowPositionInfo)

	output += "___Result for file: " + f.Name() + "___\n"

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

	p.RegisterEventHandler(func(e events.PlayerConnect) {
		if isInRound && e.Player.Team == 1 {
			spectators = append(spectators, e.Player)
		}
	})

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
			panic(err)
		}

		if isInRound {
			analyzeCamPos(p.GameState(), &spectators, positionsByNameAndPosKey, positionsListByName)
		}
	}

	output += fmt.Sprintf("__Result for static positions__\n")
	for name, positionMap := range positionsByNameAndPosKey {
		for _, positionInfo := range positionMap {
			if positionInfo.times > 200 {
				output += fmt.Sprintf("%s has been in this pos %d times\n Info: %#v\n", name, positionInfo.times, positionInfo)
			}
		}
	}

	roundNumberFlagsByName := analyzeFollowPositions(positionsListByName)

	output += fmt.Sprintf("\n__Result for third person view__\n")
	for name, flagsByRoundNumber := range roundNumberFlagsByName {
		for roundNumber, flags := range flagsByRoundNumber {
			if flags > 200 {
				output += fmt.Sprintf("%s has %d flags in round %d \n", name, flags, roundNumber)
			}
		}
	}

	output += "\n\n\n"
	
	return output
}

func analyzeFollowPositions(followPositionsByName map[string][]FollowPositionInfo) map[string]map[int]int {
	roundNumberFlagByName := make(map[string]map[int]int)

	for name, infos := range followPositionsByName {
		roundNumberFlagByName[name] = make(map[int]int)

		for _, info := range infos {
			roundNumber := info.ctScore + info.tScore + 1

			if info.xDiff + info.yDiff > 30 {
				roundNumberFlagByName[name][roundNumber]++
			}
		}
	}

	return roundNumberFlagByName
}

func analyzeCamPos(gameState dem.GameState, spectators *[]*common.Player, positionsByNameAndPosKey map[string]map[string]*StaticPositionInfo, positionsListByName map[string][]FollowPositionInfo) {
	for _, spectator := range *spectators {
		activeSpectator := gameState.Participants().ByUserID()[spectator.UserID]

		if activeSpectator == nil {
			continue
		}

		if !activeSpectator.IsConnected {
			continue
		}

		spectatorPosition := activeSpectator.Position()

		isPlayerPosition := false
		for _, player := range gameState.Participants().Playing() {

			posDistance := player.Position().Distance(spectatorPosition)

			if posDistance < 1 && player.IsAlive() {
				xDiff := calculateDiff(player.ViewDirectionX(), activeSpectator.ViewDirectionX())
				yDiff := calculateDiff(player.ViewDirectionY(), activeSpectator.ViewDirectionY())

				if xDiff > 1 || yDiff > 1 {
					positionsListByName[spectator.Name] = append(positionsListByName[spectator.Name], FollowPositionInfo{
						position:      spectatorPosition,
						spectatorName: spectator.Name,
						playerName:    player.Name,
						ctScore:       gameState.TeamCounterTerrorists().Score(),
						tScore:        gameState.TeamTerrorists().Score(),
						tick:          gameState.IngameTick(),
						xDiff:         xDiff,
						yDiff:         yDiff,
					})
				}

				isPlayerPosition = true
				break
			}
		}

		if isPlayerPosition {
			continue
		}

		if positionsByNameAndPosKey[spectator.Name] == nil {
			positionsByNameAndPosKey[spectator.Name] = make(map[string]*StaticPositionInfo)
		}

		posKey := fmt.Sprintf("%s%d%d", spectatorPosition, gameState.TeamTerrorists().Score(), gameState.TeamCounterTerrorists().Score())
		info, ok := positionsByNameAndPosKey[spectator.Name][posKey]

		if !ok {
			positionsByNameAndPosKey[spectator.Name][posKey] = &StaticPositionInfo{
				position:  spectatorPosition,
				steamId:   fmt.Sprintf("%d", spectator.SteamID64),
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

func calculateDiff(a1 float32, a2 float32) int {
	aDiff := absInt(int(a1 - a2))

	if aDiff > 180 {
		return absInt(aDiff - 360)
	} else {
		return aDiff
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

type FollowPositionInfo struct {
	position      r3.Vector
	spectatorName string
	playerName    string
	distance      float64
	ctScore       int
	tScore        int
	tick          int
	xDiff         int
	yDiff         int
}

type StaticPositionInfo struct {
	position r3.Vector
	steamId string
	times int
	ctScore int
	tScore int
	firstTick int
	lastTick int
}

